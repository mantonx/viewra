package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/utils"
)

// Resume resumes a previously paused scan
func (fs *FileScanner) Resume(libraryID uint) error {
	// Load the scan job with all necessary info
	var scanJob database.ScanJob
	if err := fs.db.Preload("Library").First(&scanJob, fs.jobID).Error; err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}
	
	// Verify the library matches
	if scanJob.LibraryID != libraryID {
		return fmt.Errorf("library ID mismatch: job is for library %d, but tried to resume for library %d", 
			scanJob.LibraryID, libraryID)
	}
	
	fs.scanJob = &scanJob
	fs.stopChan = make(chan struct{}) // Create a new stop channel
	fs.stopped = false
	
	// Update job status to running
	now := time.Now()
	scanJob.Status = "running"
	scanJob.ResumedAt = &now
	scanJob.CompletedAt = nil
	if err := fs.db.Save(&scanJob).Error; err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	
	// Start scanning - this will be called from the manager's goroutine
	fs.resumeDirectory()
	
	return nil
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
		}
		
		filesProcessed++
		progress := int((float64(filesProcessed) / float64(totalFiles)) * 100)
		fs.updateJobProgress(progress, totalFiles, filesProcessed)
		
		return nil
	})
	
	if err != nil {
		fs.updateJobError(err.Error())
	}
}
