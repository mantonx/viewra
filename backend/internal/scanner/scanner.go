package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/metadata"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// FileScanner handles recursive scanning of media directories
type FileScanner struct {
	db           *gorm.DB
	jobID        uint
	scanJob      *database.ScanJob
	mu           sync.RWMutex
	stopChan     chan struct{}
	stopped      bool
	pathResolver *utils.PathResolver
	eventBus     events.EventBus
}

// NewFileScanner creates a new file scanner instance
func NewFileScanner(db *gorm.DB, jobID uint, eventBus events.EventBus) *FileScanner {
	return &FileScanner{
		db:           db,
		jobID:        jobID,
		stopChan:     make(chan struct{}),
		pathResolver: utils.NewPathResolver(),
		eventBus:     eventBus,
	}
}

// Start begins the scanning process for a specific library
func (fs *FileScanner) Start(libraryID uint) error {
	// Load the scan job
	var scanJob database.ScanJob
	if err := fs.db.Preload("Library").First(&scanJob, fs.jobID).Error; err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}
	
	fs.scanJob = &scanJob
	
	// Update job status to running
	now := time.Now()
	scanJob.Status = "running"
	scanJob.StartedAt = &now
	if err := fs.db.Save(&scanJob).Error; err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	
	// Start scanning - this will be called from the manager's goroutine
	fs.scanDirectory()
	
	return nil
}

// Pause pauses the scanning process
func (fs *FileScanner) Pause() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if !fs.stopped {
		close(fs.stopChan)
		fs.stopped = true
	}
}

// scanDirectory performs the actual directory scanning
func (fs *FileScanner) scanDirectory() {
	defer func() {
		// Only update job completion status if it wasn't paused
		var currentJob database.ScanJob
		if err := fs.db.First(&currentJob, fs.jobID).Error; err == nil {
			if currentJob.Status != "paused" {
				// Update job completion status
				now := time.Now()
				currentJob.CompletedAt = &now
				if currentJob.Status == "running" {
					currentJob.Status = "completed"
					currentJob.Progress = 100
				}
				fs.db.Save(&currentJob)
			}
		}
	}()
	
	libraryPath := fs.scanJob.Library.Path
	
	// Resolve the library path using the path resolver
	basePath, err := fs.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		fs.updateJobError(fmt.Sprintf("Directory does not exist: %s", libraryPath))
		return
	}
	
	fmt.Printf("Resolved library path to: %s\n", basePath)
	
	// First pass: count total files to scan
	var totalFiles int
	filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}
		
		if d.IsDir() {
			return nil
		}
		
		if utils.IsMediaFile(path) {
			totalFiles++
		}
		
		return nil
	})
	
	fs.updateJobProgress(0, totalFiles, 0)
	
	// Second pass: process files
	var processedFiles int
	
	fmt.Printf("Starting scan of directory: %s\n", basePath)
	scanErr := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		// Check if we should pause
		select {
		case <-fs.stopChan:
			return fmt.Errorf("scan paused")
		default:
		}
		
		if err != nil {
			// Log error but continue scanning
			fmt.Printf("Error accessing %s: %v\n", path, err)
			return nil
		}
		
		if d.IsDir() {
			return nil
		}
		
		if !utils.IsMediaFile(path) {
			return nil
		}
		
		// Process the media file
		if err := fs.processMediaFile(path); err != nil {
			fmt.Printf("Error processing %s: %v\n", path, err)
		}
		
		processedFiles++
		progress := int((float64(processedFiles) / float64(totalFiles)) * 100)
		fs.updateJobProgress(progress, totalFiles, processedFiles)
		
		return nil
	})
	
	if scanErr != nil {
		fs.updateJobError(scanErr.Error())
	}
}

// processMediaFile processes a single media file
func (fs *FileScanner) processMediaFile(filePath string) error {
	// Resolve the file path using the path resolver
	actualPath, err := fs.pathResolver.ResolvePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to find valid path for file: %s", filePath)
	}
	
	fileInfo, err := os.Stat(actualPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Calculate file hash using utility
	hash, err := utils.CalculateFileHash(actualPath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	
	// Check if file already exists in database
	var existingFile database.MediaFile
	err = fs.db.Where("path = ? OR path = ?", filePath, actualPath).First(&existingFile).Error
	
	now := time.Now()
	
	if err == gorm.ErrRecordNotFound {
		// File doesn't exist, create new record
		mediaFile := database.MediaFile{
			Path:      actualPath,
			Size:      fileInfo.Size(),
			Hash:      hash,
			LibraryID: fs.scanJob.LibraryID,
			LastSeen:  now,
		}
		
		if err := fs.db.Create(&mediaFile).Error; err != nil {
			return fmt.Errorf("failed to create media file record: %w", err)
		}
		
		// Extract music metadata if this is a music file
		if metadata.IsMusicFile(actualPath) {
			fmt.Printf("Extracting music metadata for: %s\n", actualPath)
			musicMeta, err := metadata.ExtractMusicMetadata(actualPath, &mediaFile)
			if err != nil {
				fmt.Printf("Warning: failed to extract metadata for %s: %v\n", actualPath, err)
			} else {
				if err := fs.db.Create(musicMeta).Error; err != nil {
					fmt.Printf("Warning: failed to save metadata for %s: %v\n", actualPath, err)
				} else {
					fmt.Printf("Successfully saved music metadata for: %s\n", actualPath)
				}
			}
		}
		
		// Update the bytes processed in the scan job
		fs.mu.Lock()
		fs.scanJob.BytesProcessed += fileInfo.Size()
		fs.mu.Unlock()
		
	} else if err != nil {
		return fmt.Errorf("database error: %w", err)
	} else {
		// File exists, update last seen and verify hash
		existingFile.LastSeen = now
		needsUpdate := false
		
		if existingFile.Hash != hash {
			existingFile.Hash = hash
			existingFile.Size = fileInfo.Size()
			needsUpdate = true
		}
		
		if err := fs.db.Save(&existingFile).Error; err != nil {
			return fmt.Errorf("failed to update media file record: %w", err)
		}
		
		// Check if we need to extract/update music metadata
		if metadata.IsMusicFile(actualPath) && needsUpdate {
			// Delete existing metadata if it exists
			fs.db.Where("media_file_id = ?", existingFile.ID).Delete(&database.MusicMetadata{})
			
			fmt.Printf("Updating music metadata for: %s\n", actualPath)
			// Extract new metadata
			musicMeta, err := metadata.ExtractMusicMetadata(actualPath, &existingFile)
			if err != nil {
				fmt.Printf("Warning: failed to extract metadata for %s: %v\n", actualPath, err)
			} else {
				if err := fs.db.Create(musicMeta).Error; err != nil {
					fmt.Printf("Warning: failed to save metadata for %s: %v\n", actualPath, err)
				} else {
					fmt.Printf("Successfully updated music metadata for: %s\n", actualPath)
				}
			}
		}
	}
	
	return nil
}

// updateJobProgress updates the scan job progress
func (fs *FileScanner) updateJobProgress(progress, filesFound, filesProcessed int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	fs.scanJob.Progress = progress
	fs.scanJob.FilesFound = filesFound
	fs.scanJob.FilesProcessed = filesProcessed
	
	// Calculate bytes processed (sum of all file sizes)
	if fs.scanJob.BytesProcessed == 0 && filesProcessed > 0 {
		var totalBytes int64
		fs.db.Model(&database.MediaFile{}).
			Where("library_id = ?", fs.scanJob.LibraryID).
			Select("COALESCE(SUM(size), 0)").
			Scan(&totalBytes)
		
		fs.scanJob.BytesProcessed = totalBytes
	}
	
	fs.db.Save(fs.scanJob)
}

// updateJobError updates the scan job with an error
func (fs *FileScanner) updateJobError(errorMsg string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	fs.scanJob.Status = "failed"
	fs.scanJob.ErrorMessage = errorMsg
	now := time.Now()
	fs.scanJob.CompletedAt = &now
	fs.db.Save(fs.scanJob)
}
