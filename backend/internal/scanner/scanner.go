package scanner

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/viewra/internal/database"
	"github.com/yourusername/viewra/internal/metadata"
	"gorm.io/gorm"
)

// MediaExtensions contains supported media file extensions
var MediaExtensions = map[string]bool{
	// Video formats
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".3gp":  true,
	".ogv":  true,
	
	// Audio formats
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".aac":  true,
	".ogg":  true,
	".wma":  true,
	".m4a":  true,
	".opus": true,
	".aiff": true,
}

// FileScanner handles recursive scanning of media directories
type FileScanner struct {
	db       *gorm.DB
	jobID    uint
	scanJob  *database.ScanJob
	mu       sync.RWMutex
	stopChan chan struct{}
	stopped  bool
}

// NewFileScanner creates a new file scanner instance
func NewFileScanner(db *gorm.DB, jobID uint) *FileScanner {
	return &FileScanner{
		db:       db,
		jobID:    jobID,
		stopChan: make(chan struct{}),
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
	
	// Check if directory exists - try multiple path variants to support Docker vs local dev
	basePath := libraryPath
	pathVariants := []string{
		libraryPath, // Original path
	}
	
	// Add common Docker to local path mappings
	if strings.HasPrefix(libraryPath, "/app/") {
		// If path starts with /app/, try without it (for local dev)
		pathVariants = append(pathVariants, strings.TrimPrefix(libraryPath, "/app"))
		// Also try with current directory
		pathVariants = append(pathVariants, filepath.Join(".", strings.TrimPrefix(libraryPath, "/app")))
	} else {
		// If path is relative or absolute but not /app, try with /app prefix (for Docker)
		pathVariants = append(pathVariants, filepath.Join("/app", libraryPath))
	}
	
	// Try workspace-relative paths with different combinations
	pathVariants = append(pathVariants, filepath.Join("/home/fictional/Projects/viewra/backend", libraryPath))
	pathVariants = append(pathVariants, filepath.Join("/home/fictional/Projects/viewra", libraryPath))
	pathVariants = append(pathVariants, libraryPath[2:]) // Remove ./ prefix if exists
	
	// For testing - try current directory
	pwd, err := os.Getwd()
	if err == nil {
		fmt.Printf("Current working directory: %s\n", pwd)
		pathVariants = append(pathVariants, filepath.Join(pwd, libraryPath))
		
		// If we're in the backend directory, try test-music directly
		if strings.HasSuffix(pwd, "/backend") {
			pathVariants = append(pathVariants, filepath.Join(pwd, "data/test-music"))
		}
	}
	
	// Log path variants we're trying
	fmt.Printf("Trying path variants for %s:\n", libraryPath)
	for _, path := range pathVariants {
		fmt.Printf("  - %s\n", path)
	}
	
	// Check each variant
	dirFound := false
	for _, path := range pathVariants {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Found valid path: %s\n", path)
			basePath = path
			dirFound = true
			break
		}
	}
	
	// Hardcoded check specifically for test-music directory for testing purposes
	testMusicPath := "/home/fictional/Projects/viewra/backend/data/test-music"
	if strings.Contains(libraryPath, "test-music") {
		if _, err := os.Stat(testMusicPath); err == nil {
			fmt.Printf("Found test music directory at: %s\n", testMusicPath)
			basePath = testMusicPath
			dirFound = true
		}
	}
	
	if !dirFound {
		fs.updateJobError(fmt.Sprintf("Directory does not exist in any variant: %s", libraryPath))
		return
	}
	
	// Update library path to the working one
	libraryPath = basePath
	
	// First pass: count total files to scan
	var totalFiles int
	filepath.WalkDir(libraryPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}
		
		if d.IsDir() {
			return nil
		}
		
		if fs.isMediaFile(path) {
			totalFiles++
		}
		
		return nil
	})
	
	fs.updateJobProgress(0, totalFiles, 0)
	
	// Second pass: process files
	var processedFiles int
	
	fmt.Printf("Starting scan of directory: %s\n", libraryPath)
	scanErr := filepath.WalkDir(libraryPath, func(path string, d os.DirEntry, err error) error {
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
		
		if !fs.isMediaFile(path) {
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
	// Try to open the file, using path variants if necessary
	var fileInfo os.FileInfo
	var err error
	var actualPath string

	// Check if file exists directly
	fileInfo, err = os.Stat(filePath)
	if err == nil {
		actualPath = filePath
	} else {
		// Try path variants
		pathVariants := []string{filePath}
		
		// Add common Docker to local path mappings
		if strings.HasPrefix(filePath, "/app/") {
			pathVariants = append(pathVariants, strings.TrimPrefix(filePath, "/app"))
			pathVariants = append(pathVariants, filepath.Join(".", strings.TrimPrefix(filePath, "/app")))
		} else {
			pathVariants = append(pathVariants, filepath.Join("/app", filePath))
		}
		
		// Try workspace-relative paths with different combinations
		pathVariants = append(pathVariants, filepath.Join("/home/fictional/Projects/viewra/backend", filePath))
		pathVariants = append(pathVariants, filepath.Join("/home/fictional/Projects/viewra", filePath))
		
		// For testing - add current directory paths
		pwd, err := os.Getwd()
		if err == nil {
			pathVariants = append(pathVariants, filepath.Join(pwd, filePath))
			
			// If filePath has "data/test-music" in it, try to construct a path directly
			if strings.Contains(filePath, "data/test-music") {
				parts := strings.Split(filePath, "data/test-music")
				if len(parts) > 1 {
					relPath := "data/test-music" + parts[len(parts)-1]
					pathVariants = append(pathVariants, filepath.Join(pwd, relPath))
				}
			}
		}
		
		for _, path := range pathVariants {
			if fi, err := os.Stat(path); err == nil {
				fileInfo = fi
				actualPath = path
				fmt.Printf("Found valid file path: %s\n", path)
				break
			}
		}
		
		if actualPath == "" {
			return fmt.Errorf("failed to find valid path for file: %s", filePath)
		}
	}
	
	// Calculate file hash
	hash, err := fs.calculateFileHash(actualPath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	
	// Check if file already exists in database - check both the original and actual path
	var existingFile database.MediaFile
	err = fs.db.Where("path = ? OR path = ?", filePath, actualPath).First(&existingFile).Error
	
	now := time.Now()
	
	if err == gorm.ErrRecordNotFound {
		// File doesn't exist, create new record
		mediaFile := database.MediaFile{
			Path:      actualPath, // Use the actual path that works in this environment
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
				// Log error but don't fail the scan
				fmt.Printf("Warning: failed to extract metadata for %s: %v\n", actualPath, err)
			} else {
				// Save music metadata
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

// isMediaFile checks if a file has a supported media extension
func (fs *FileScanner) isMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return MediaExtensions[ext]
}

// calculateFileHash calculates SHA1 hash of a file
func (fs *FileScanner) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
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
