package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// FileScanner represents the legacy file scanner implementation
type FileScanner struct {
	db           *gorm.DB
	jobID        uint
	scanJob      *database.ScanJob
	stopChan     chan struct{}
	pathResolver *utils.PathResolver
}

// NewFileScanner creates a new legacy file scanner
func NewFileScanner(db *gorm.DB, jobID uint) *FileScanner {
	return &FileScanner{
		db:           db,
		jobID:        jobID,
		stopChan:     make(chan struct{}),
		pathResolver: utils.NewPathResolver(),
	}
}

// resumeDirectory performs the resumed directory scanning
func (fs *FileScanner) resumeDirectory() {
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

	fmt.Printf("Resumed scanning at resolved path: %s\n", basePath)

	// Get the number of files already processed
	filesProcessed := fs.scanJob.FilesProcessed
	totalFiles := fs.scanJob.FilesFound

	// If there are no files found yet, we need to count them first
	// Also recount if we're resuming a scan that might have been paused for a long time
	// This helps with accuracy if files were added/removed while scan was paused
	totalFiles = 0
	filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		// Check if we should stop
		select {
		case <-fs.stopChan:
			return fmt.Errorf("scan paused during file counting")
		default:
		}

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

	// Update the job with the latest file count
	var scanJob database.ScanJob
	if err := fs.db.First(&scanJob, fs.jobID).Error; err == nil {
		scanJob.FilesFound = totalFiles
		fs.db.Save(&scanJob)
	}

	// Calculate and update progress
	var progress int
	if totalFiles > 0 {
		progress = int((float64(filesProcessed) / float64(totalFiles)) * 100)
	} else {
		progress = 0
	}
	fs.updateJobProgress(progress, totalFiles, filesProcessed)

	// Get a list of already processed files to skip them efficiently
	processedFiles := make(map[string]bool)

	if filesProcessed > 0 {
		// Get the list of files already in the database for this library
		var existingFiles []database.MediaFile
		if err := fs.db.Where("library_id = ?", fs.scanJob.LibraryID).Find(&existingFiles).Error; err == nil {
			for _, file := range existingFiles {
				processedFiles[file.Path] = true
			}
		}
	}

	// Process files, skipping the ones we've already processed
	err = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		// Check if we should stop
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

		// Skip files we've already processed if resuming
		if processedFiles[path] {
			return nil
		}

		// Process the media file
		if err := fs.processMediaFile(path); err != nil {
			fmt.Printf("Error processing %s: %v\n", path, err)
			// Update job with the error encountered
			fs.updateJobError(err.Error())
		} else {
			// Increment processed files count
			filesProcessed++
		}

		// Update progress periodically
		if filesProcessed%100 == 0 {
			var progress int
			if totalFiles > 0 {
				progress = int((float64(filesProcessed) / float64(totalFiles)) * 100)
			}
			fs.updateJobProgress(progress, totalFiles, filesProcessed)
		}

		return nil
	})

	if err != nil {
		fs.updateJobError(err.Error())
	}
}

// processMediaFile handles the processing of a single media file
func (fs *FileScanner) processMediaFile(filePath string) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Use optimized hash calculation for large files
	var hash string
	if fileInfo.Size() > 10*1024*1024 { // 10MB threshold
		hash, err = utils.CalculateFileHashSampled(filePath, fileInfo.Size())
	} else {
		hash, err = utils.CalculateFileHash(filePath)
	}
	
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}
	
	// Check if file already exists in the database
	var mediaFile database.MediaFile
	if err := fs.db.Where("hash = ?", hash).First(&mediaFile).Error; err == nil {
		// File already exists, update last seen
		mediaFile.LastSeen = time.Now()
		return fs.db.Save(&mediaFile).Error
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}
	
	// File is new, create a new record
	mediaFile = database.MediaFile{
		Hash:      hash,
		Path:      filePath,
		Size:      fileInfo.Size(),
		LibraryID: fs.scanJob.LibraryID,
		LastSeen:  time.Now(),
	}
	
	return fs.db.Create(&mediaFile).Error
}

// updateJobProgress updates the progress of the scan job
func (fs *FileScanner) updateJobProgress(progress int, totalFiles int, filesProcessed int) {
	if err := fs.db.Model(&database.ScanJob{}).Where("id = ?", fs.jobID).
		Updates(map[string]interface{}{
			"progress":        progress,
			"files_found":     totalFiles,
			"files_processed": filesProcessed,
			"updated_at":      time.Now(),
		}).Error; err != nil {
		fmt.Printf("Failed to update job progress: %v\n", err)
	}
}

// updateJobError updates the error message of a scan job
func (fs *FileScanner) updateJobError(errorMsg string) {
	if err := fs.db.Model(&database.ScanJob{}).Where("id = ?", fs.jobID).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errorMsg,
			"updated_at":    time.Now(),
		}).Error; err != nil {
		fmt.Printf("Failed to update job error: %v\n", err)
	}
}