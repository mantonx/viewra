package mediamodule

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/metadata"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// FileProcessor handles media file processing operations
type FileProcessor struct {
	db           *gorm.DB
	eventBus     events.EventBus
	initialized  bool
	mutex        sync.RWMutex
	
	// Processing queues and workers
	processingQueue chan *ProcessJob
	workerCount     int
	activeJobs      map[string]*ProcessJob
	jobsMutex       sync.RWMutex
}

// ProcessJob represents a file processing job
type ProcessJob struct {
	ID           string    `json:"id"`
	MediaFileID  uint      `json:"media_file_id"`
	FilePath     string    `json:"file_path"`
	Status       string    `json:"status"` // pending, processing, completed, failed
	Progress     float64   `json:"progress"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// ProcessingStats represents file processor statistics
type ProcessingStats struct {
	ActiveJobs    int       `json:"active_jobs"`
	CompletedJobs int       `json:"completed_jobs"`
	FailedJobs    int       `json:"failed_jobs"`
	QueuedJobs    int       `json:"queued_jobs"`
	Uptime        time.Duration `json:"uptime"`
	StartTime     time.Time `json:"start_time"`
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(db *gorm.DB, eventBus events.EventBus) *FileProcessor {
	return &FileProcessor{
		db:             db,
		eventBus:       eventBus,
		processingQueue: make(chan *ProcessJob, 100), // Buffer size of 100
		workerCount:     3, // Default to 3 workers
		activeJobs:      make(map[string]*ProcessJob),
	}
}

// Initialize initializes the file processor
func (fp *FileProcessor) Initialize() error {
	log.Println("INFO: Initializing file processor")
	
	// Start worker goroutines
	for i := 0; i < fp.workerCount; i++ {
		go fp.processWorker(i)
	}
	
	fp.initialized = true
	log.Println("INFO: File processor initialized successfully")
	return nil
}

// ProcessFile processes a media file with the given ID
func (fp *FileProcessor) ProcessFile(mediaFileID uint) (string, error) {
	if !fp.initialized {
		return "", fmt.Errorf("file processor not initialized")
	}
	
	// Get file information from database
	var mediaFile database.MediaFile
	if err := fp.db.First(&mediaFile, mediaFileID).Error; err != nil {
		return "", fmt.Errorf("failed to find media file: %w", err)
	}
	
	// Generate a unique job ID
	jobID := fmt.Sprintf("job-%d-%d", mediaFileID, time.Now().UnixNano())
	
	// Create job
	job := &ProcessJob{
		ID:          jobID,
		MediaFileID: mediaFileID,
		FilePath:    mediaFile.Path,
		Status:      "pending",
		Progress:    0,
		StartedAt:   time.Now(),
	}
	
	// Add to active jobs
	fp.jobsMutex.Lock()
	fp.activeJobs[jobID] = job
	fp.jobsMutex.Unlock()
	
	// Submit to processing queue
	select {
	case fp.processingQueue <- job:
		log.Printf("INFO: File processing job %s queued for media file ID %d", jobID, mediaFileID)
	default:
		// Queue full, handle gracefully
		fp.jobsMutex.Lock()
		delete(fp.activeJobs, jobID)
		fp.jobsMutex.Unlock()
		return "", fmt.Errorf("processing queue full, try again later")
	}
	
	// Publish job queued event
	if fp.eventBus != nil {
		event := events.NewSystemEvent(
			"media.file.processing.queued",
			"File Processing Queued",
			fmt.Sprintf("Processing job %s queued for file ID %d", jobID, mediaFileID),
		)
		event.Data = map[string]interface{}{
			"jobID":       jobID,
			"mediaFileID": mediaFileID,
			"filePath":    mediaFile.Path,
		}
		fp.eventBus.PublishAsync(event)
	}
	
	return jobID, nil
}

// GetJobStatus returns the status of a processing job
func (fp *FileProcessor) GetJobStatus(jobID string) (*ProcessJob, error) {
	if !fp.initialized {
		return nil, fmt.Errorf("file processor not initialized")
	}
	
	fp.jobsMutex.RLock()
	job, exists := fp.activeJobs[jobID]
	fp.jobsMutex.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("job not found")
	}
	
	return job, nil
}

// GetStats returns statistics about the file processor
func (fp *FileProcessor) GetStats() *ProcessingStats {
	stats := &ProcessingStats{
		StartTime: time.Now().Add(-1 * time.Hour), // Placeholder uptime
		Uptime:    time.Hour,                      // Placeholder 1 hour uptime
	}
	
	fp.jobsMutex.RLock()
	for _, job := range fp.activeJobs {
		switch job.Status {
		case "pending", "processing":
			stats.ActiveJobs++
		case "completed":
			stats.CompletedJobs++
		case "failed":
			stats.FailedJobs++
		}
	}
	fp.jobsMutex.RUnlock()
	
	// Get queue depth
	stats.QueuedJobs = len(fp.processingQueue)
	
	return stats
}

// processWorker handles processing jobs from the queue
func (fp *FileProcessor) processWorker(workerID int) {
	log.Printf("INFO: Starting file processor worker %d", workerID)
	
	for job := range fp.processingQueue {
		log.Printf("INFO: Worker %d processing job %s for file %s", workerID, job.ID, job.FilePath)
		
		// Update job status
		fp.jobsMutex.Lock()
		if j, exists := fp.activeJobs[job.ID]; exists {
			j.Status = "processing"
		}
		fp.jobsMutex.Unlock()
		
		// Publish job started event
		if fp.eventBus != nil {
			event := events.NewSystemEvent(
				"media.file.processing.started",
				"File Processing Started",
				fmt.Sprintf("Processing started for job %s", job.ID),
			)
			fp.eventBus.PublishAsync(event)
		}
		
		// Process the file
		err := fp.processFileJob(job)
		
		// Update job status based on result
		fp.jobsMutex.Lock()
		if jobPtr, exists := fp.activeJobs[job.ID]; exists {
			jobPtr.CompletedAt = time.Now()
			if err != nil {
				jobPtr.Status = "failed"
				jobPtr.ErrorMessage = err.Error()
				log.Printf("ERROR: Job %s failed: %v", job.ID, err)
			} else {
				jobPtr.Status = "completed"
				jobPtr.Progress = 100
				log.Printf("INFO: Job %s completed successfully", job.ID)
			}
		}
		fp.jobsMutex.Unlock()
		
		// Publish job completed event
		if fp.eventBus != nil {
			var title, description string
			var eventType events.EventType
			if err != nil {
				eventType = "media.file.processing.failed"
				title = "File Processing Failed"
				description = fmt.Sprintf("Processing failed for job %s: %v", job.ID, err)
			} else {
				eventType = "media.file.processing.completed"
				title = "File Processing Completed"
				description = fmt.Sprintf("Processing completed for job %s", job.ID)
			}
			
			event := events.NewSystemEvent(eventType, title, description)
			fp.eventBus.PublishAsync(event)
		}
	}
}

// processFileJob processes a single file job
func (fp *FileProcessor) processFileJob(job *ProcessJob) error {
	// Check if file exists
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", job.FilePath)
	}
	
	// Get file from database
	var mediaFile database.MediaFile
	if err := fp.db.First(&mediaFile, job.MediaFileID).Error; err != nil {
		return fmt.Errorf("failed to find media file in database: %w", err)
	}
	
	// Update progress
	fp.updateJobProgress(job.ID, 10)
	
	// Calculate file hash for verification
	hash, err := utils.CalculateFileHash(job.FilePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}
	
	// Update progress
	fp.updateJobProgress(job.ID, 30)
	
	// Update hash in database if needed
	if mediaFile.Hash != hash {
		mediaFile.Hash = hash
		if err := fp.db.Save(&mediaFile).Error; err != nil {
			return fmt.Errorf("failed to update file hash: %w", err)
		}
	}
	
	// Process based on file type
	switch {
	case metadata.IsMusicFile(job.FilePath):
		// Update progress
		fp.updateJobProgress(job.ID, 50)
		
		// Process music file
		if err := fp.processMusicFile(job, &mediaFile); err != nil {
			return fmt.Errorf("failed to process music file: %w", err)
		}
		
	// TODO: Add processing for other file types (video, images, etc.)
	default:
		// Generic processing for unknown file types
		log.Printf("WARNING: Unknown file type for %s, performing generic processing", job.FilePath)
	}
	
	// Update progress
	fp.updateJobProgress(job.ID, 90)
	
	// Final updates to the media file record
	mediaFile.LastSeen = time.Now()
	if err := fp.db.Save(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to update media file: %w", err)
	}
	
	// Update progress
	fp.updateJobProgress(job.ID, 100)
	
	return nil
}

// processMusicFile processes a music file and extracts metadata
func (fp *FileProcessor) processMusicFile(job *ProcessJob, mediaFile *database.MediaFile) error {
	log.Printf("INFO: Processing music file: %s", job.FilePath)
	
	// Delete existing metadata if it exists
	if err := fp.db.Where("media_file_id = ?", mediaFile.ID).Delete(&database.MusicMetadata{}).Error; err != nil {
		log.Printf("WARNING: Failed to delete existing music metadata: %v", err)
	}
	
	// Extract new metadata
	musicMeta, err := metadata.ExtractMusicMetadata(job.FilePath, mediaFile)
	if err != nil {
		return fmt.Errorf("failed to extract music metadata: %w", err)
	}
	
	// Save new metadata to database
	if err := fp.db.Create(musicMeta).Error; err != nil {
		return fmt.Errorf("failed to save music metadata: %w", err)
	}
	
	log.Printf("INFO: Successfully processed music file: %s", job.FilePath)
	return nil
}

// updateJobProgress updates the progress of a job
func (fp *FileProcessor) updateJobProgress(jobID string, progress float64) {
	fp.jobsMutex.Lock()
	if job, exists := fp.activeJobs[jobID]; exists {
		job.Progress = progress
	}
	fp.jobsMutex.Unlock()
}

// Shutdown gracefully shuts down the file processor
func (fp *FileProcessor) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down file processor")
	
	// Close processing queue
	close(fp.processingQueue)
	
	// Wait for context or timeout
	select {
	case <-ctx.Done():
		log.Println("INFO: Context canceled while shutting down file processor")
	case <-time.After(5 * time.Second):
		log.Println("INFO: Timeout while waiting for file processor to shut down")
	}
	
	fp.initialized = false
	log.Println("INFO: File processor shutdown complete")
	return nil
}