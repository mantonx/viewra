package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/metadata"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

const (
	// Number of worker goroutines for file processing
	DefaultWorkers = 4
	// Buffer size for file channel
	FileChannelBuffer = 100
	// Batch size for database operations
	BatchSize = 50
)

// ParallelFileScanner handles concurrent scanning of media directories
type ParallelFileScanner struct {
	db           *gorm.DB
	jobID        uint
	scanJob      *database.ScanJob
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	pathResolver *utils.PathResolver
	workers      int
	eventBus     events.EventBus
}

// FileTask represents a file to be processed
type FileTask struct {
	Path     string
	FileInfo os.FileInfo
}

// NewParallelFileScanner creates a new parallel file scanner instance
func NewParallelFileScanner(db *gorm.DB, jobID uint, eventBus events.EventBus) *ParallelFileScanner {
	ctx, cancel := context.WithCancel(context.Background())
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}
	
	return &ParallelFileScanner{
		db:           db,
		jobID:        jobID,
		ctx:          ctx,
		cancel:       cancel,
		pathResolver: utils.NewPathResolver(),
		workers:      workers,
		eventBus:     eventBus,
	}
}

// Start begins the parallel scanning process for a specific library
func (pfs *ParallelFileScanner) Start(libraryID uint) error {
	// Load the scan job
	var scanJob database.ScanJob
	if err := pfs.db.Preload("Library").First(&scanJob, pfs.jobID).Error; err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}
	
	pfs.scanJob = &scanJob
	
	// Update job status to running
	now := time.Now()
	scanJob.Status = "running"
	scanJob.StartedAt = &now
	if err := pfs.db.Save(&scanJob).Error; err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	
	// Start parallel scanning
	return pfs.scanDirectoryParallel()
}

// Pause pauses the scanning process
func (pfs *ParallelFileScanner) Pause() {
	pfs.cancel()
	
	// Publish pause event
	if pfs.eventBus != nil {
		pauseEvent := events.NewSystemEvent(
			events.EventScanPaused,
			"Media Scan Paused", 
			fmt.Sprintf("Scan job #%d has been paused", pfs.jobID),
		)
		
		// Get current scan stats
		pfs.mu.RLock()
		scanJob := pfs.scanJob
		pfs.mu.RUnlock()
		
		if scanJob != nil {
			pauseEvent.Data = map[string]interface{}{
				"libraryId":      scanJob.LibraryID,
				"scanJobId":      pfs.jobID,
				"progress":       scanJob.Progress,
				"filesFound":     scanJob.FilesFound,
				"filesProcessed": scanJob.FilesProcessed,
			}
		}
		
		pfs.eventBus.PublishAsync(pauseEvent)
	}
}

// scanDirectoryParallel performs parallel directory scanning
func (pfs *ParallelFileScanner) scanDirectoryParallel() error {
	startTime := time.Now() // Track start time for duration calculation
	
	defer func() {
		// Update job completion status if it wasn't paused
		var currentJob database.ScanJob
		if err := pfs.db.First(&currentJob, pfs.jobID).Error; err == nil {
			if currentJob.Status != "paused" {
				now := time.Now()
				currentJob.CompletedAt = &now
				if currentJob.Status == "running" {
					currentJob.Status = "completed"
					currentJob.Progress = 100
				}
				pfs.db.Save(&currentJob)
			}
		}
	}()

	libraryPath := pfs.scanJob.Library.Path
	
	// Resolve the library path
	basePath, err := pfs.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		pfs.updateJobError(fmt.Sprintf("Directory does not exist: %s", libraryPath))
		return err
	}

	fmt.Printf("Starting parallel scan of directory: %s with %d workers\n", basePath, pfs.workers)

	// Get existing files from database for quick lookup
	existingFiles, err := pfs.getExistingFilesMap()
	if err != nil {
		pfs.updateJobError(fmt.Sprintf("Failed to load existing files: %v", err))
		return err
	}

	// Channels for worker communication
	fileChan := make(chan FileTask, FileChannelBuffer)
	resultChan := make(chan *database.MediaFile, FileChannelBuffer)
	errorChan := make(chan error, pfs.workers)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < pfs.workers; i++ {
		wg.Add(1)
		go pfs.worker(i, fileChan, resultChan, errorChan, existingFiles, &wg)
	}

	// Start result collector
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go pfs.resultCollector(resultChan, &collectorWg)

	// Walk directory and send files to workers
	var totalFiles int
	walkErr := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-pfs.ctx.Done():
			return fmt.Errorf("scan cancelled")
		default:
		}

		if err != nil {
			fmt.Printf("Error accessing %s: %v\n", path, err)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !utils.IsMediaFile(path) {
			return nil
		}

		totalFiles++
		select {
		case fileChan <- FileTask{Path: path, FileInfo: info}:
		case <-pfs.ctx.Done():
			return fmt.Errorf("scan cancelled")
		}

		return nil
	})

	// Close file channel to signal workers to finish
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()
	close(resultChan)

	// Wait for result collector to finish
	collectorWg.Wait()

	// Update final progress
	pfs.updateJobProgress(100, totalFiles, totalFiles)

	if walkErr != nil {
		pfs.updateJobError(walkErr.Error())
		
		// Publish scan failed event
		if pfs.eventBus != nil {
			failEvent := events.NewSystemEvent(
				events.EventScanFailed,
				"Media Scan Failed",
				fmt.Sprintf("Scan job #%d failed: %s", pfs.jobID, walkErr.Error()),
			)
			failEvent.Data = map[string]interface{}{
				"libraryId": pfs.scanJob.LibraryID,
				"scanJobId": pfs.jobID,
				"error": walkErr.Error(),
				"filesProcessed": totalFiles,
				"duration": time.Since(startTime).String(),
			}
			pfs.eventBus.PublishAsync(failEvent)
		}
		
		return walkErr
	}

	// Publish scan completion event
	if pfs.eventBus != nil {
		completionEvent := events.NewSystemEvent(
			events.EventScanCompleted,
			"Media Scan Completed",
			fmt.Sprintf("Completed scan job #%d, processed %d files", pfs.jobID, totalFiles),
		)
		completionEvent.Data = map[string]interface{}{
			"libraryId": pfs.scanJob.LibraryID,
			"scanJobId": pfs.jobID,
			"filesProcessed": totalFiles,
			"duration": time.Since(startTime).String(),
		}
		pfs.eventBus.PublishAsync(completionEvent)
	}

	fmt.Printf("Parallel scan completed. Processed %d files\n", totalFiles)
	return nil
}

// worker processes files from the file channel
func (pfs *ParallelFileScanner) worker(id int, fileChan <-chan FileTask, resultChan chan<- *database.MediaFile, errorChan chan<- error, existingFiles map[string]*database.MediaFile, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Worker %d started\n", id)
	
	for task := range fileChan {
		select {
		case <-pfs.ctx.Done():
			return
		default:
		}

		mediaFile, err := pfs.processFileTask(task, existingFiles)
		if err != nil {
			select {
			case errorChan <- err:
			default:
			}
			continue
		}

		if mediaFile != nil {
			select {
			case resultChan <- mediaFile:
			case <-pfs.ctx.Done():
				return
			}
		}
	}
	
	fmt.Printf("Worker %d finished\n", id)
}

// processFileTask processes a single file task
func (pfs *ParallelFileScanner) processFileTask(task FileTask, existingFiles map[string]*database.MediaFile) (*database.MediaFile, error) {
	actualPath, err := pfs.pathResolver.ResolvePath(task.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", task.Path, err)
	}

	// Check if file exists in our map
	if existingFile, exists := existingFiles[actualPath]; exists {
		// File exists - check if we need to update
		if pfs.shouldUpdateFile(existingFile, task.FileInfo) {
			return pfs.updateExistingFile(existingFile, actualPath, task.FileInfo)
		}
		// No update needed, just update last seen
		existingFile.LastSeen = time.Now()
		return existingFile, nil
	}

	// New file - create record
	return pfs.createNewFile(actualPath, task.FileInfo)
}

// shouldUpdateFile determines if a file needs to be updated based on modification time and size
func (pfs *ParallelFileScanner) shouldUpdateFile(existing *database.MediaFile, info os.FileInfo) bool {
	return existing.Size != info.Size() || existing.LastSeen.Before(info.ModTime())
}

// createNewFile creates a new media file record
func (pfs *ParallelFileScanner) createNewFile(filePath string, info os.FileInfo) (*database.MediaFile, error) {
	// Only calculate hash for new files
	hash, err := utils.CalculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	mediaFile := &database.MediaFile{
		Path:      filePath,
		Size:      info.Size(),
		Hash:      hash,
		LibraryID: pfs.scanJob.LibraryID,
		LastSeen:  time.Now(),
	}

	// Publish event for new file found
	if pfs.eventBus != nil {
		fileFoundEvent := events.NewSystemEvent(
			events.EventMediaFileFound,
			"Media File Found",
			fmt.Sprintf("Found new file: %s", filepath.Base(filePath)),
		)
		fileFoundEvent.Data = map[string]interface{}{
			"path": filePath,
			"size": info.Size(),
			"hash": hash,
			"scanJobId": pfs.jobID,
			"libraryId": pfs.scanJob.LibraryID,
			"extension": filepath.Ext(filePath),
		}
		pfs.eventBus.PublishAsync(fileFoundEvent)
	}

	return mediaFile, nil
}

// updateExistingFile updates an existing file record
func (pfs *ParallelFileScanner) updateExistingFile(existing *database.MediaFile, filePath string, info os.FileInfo) (*database.MediaFile, error) {
	// Only recalculate hash if size changed
	if existing.Size != info.Size() {
		hash, err := utils.CalculateFileHash(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate hash: %w", err)
		}
		existing.Hash = hash
	}

	existing.Size = info.Size()
	existing.LastSeen = time.Now()

	return existing, nil
}

// resultCollector collects results and performs batch database operations
func (pfs *ParallelFileScanner) resultCollector(resultChan <-chan *database.MediaFile, wg *sync.WaitGroup) {
	defer wg.Done()

	var batch []*database.MediaFile
	var processedCount int

	// Process metadata in batches asynchronously
	metadataChan := make(chan *database.MediaFile, FileChannelBuffer)
	var metadataWg sync.WaitGroup
	
	// Start metadata workers
	for i := 0; i < 2; i++ { // Use 2 workers for metadata extraction
		metadataWg.Add(1)
		go pfs.metadataWorker(metadataChan, &metadataWg)
	}

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		// Perform batch database operations
		if err := pfs.saveBatch(batch); err != nil {
			fmt.Printf("Error saving batch: %v\n", err)
		}

		// Send files for metadata extraction
		for _, file := range batch {
			if metadata.IsMusicFile(file.Path) {
				select {
				case metadataChan <- file:
				case <-pfs.ctx.Done():
					return
				}
			}
		}

		processedCount += len(batch)
		pfs.updateProcessedCount(processedCount)
		batch = batch[:0] // Clear batch
	}

	for mediaFile := range resultChan {
		batch = append(batch, mediaFile)

		if len(batch) >= BatchSize {
			flushBatch()
		}
	}

	// Flush remaining batch
	flushBatch()

	// Close metadata channel and wait for metadata workers
	close(metadataChan)
	metadataWg.Wait()

	fmt.Printf("Result collector finished. Processed %d files\n", processedCount)
}

// saveBatch saves a batch of media files to the database
func (pfs *ParallelFileScanner) saveBatch(batch []*database.MediaFile) error {
	if len(batch) == 0 {
		return nil
	}

	return pfs.db.Transaction(func(tx *gorm.DB) error {
		for _, file := range batch {
			if file.ID == 0 {
				// New file
				if err := tx.Create(file).Error; err != nil {
					return err
				}
			} else {
				// Update existing file
				if err := tx.Save(file).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// metadataWorker processes metadata extraction asynchronously
func (pfs *ParallelFileScanner) metadataWorker(metadataChan <-chan *database.MediaFile, wg *sync.WaitGroup) {
	defer wg.Done()

	for mediaFile := range metadataChan {
		select {
		case <-pfs.ctx.Done():
			return
		default:
		}

		if musicMeta, err := metadata.ExtractMusicMetadata(mediaFile.Path, mediaFile); err != nil {
			fmt.Printf("Warning: failed to extract metadata for %s: %v\n", mediaFile.Path, err)
		} else {
			if err := pfs.db.Create(musicMeta).Error; err != nil {
				fmt.Printf("Warning: failed to save metadata for %s: %v\n", mediaFile.Path, err)
			}
		}
	}
}

// getExistingFilesMap loads existing files from database into a map for quick lookup
func (pfs *ParallelFileScanner) getExistingFilesMap() (map[string]*database.MediaFile, error) {
	var files []database.MediaFile
	if err := pfs.db.Where("library_id = ?", pfs.scanJob.LibraryID).Find(&files).Error; err != nil {
		return nil, err
	}

	fileMap := make(map[string]*database.MediaFile, len(files))
	for i := range files {
		fileMap[files[i].Path] = &files[i]
	}

	return fileMap, nil
}

// updateProcessedCount updates the processed file count
func (pfs *ParallelFileScanner) updateProcessedCount(count int) {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()
	
	pfs.scanJob.FilesProcessed = count
	if pfs.scanJob.FilesFound > 0 {
		pfs.scanJob.Progress = int((float64(count) / float64(pfs.scanJob.FilesFound)) * 100)
	}
	pfs.db.Save(pfs.scanJob)
}

// updateJobProgress updates the scan job progress
func (pfs *ParallelFileScanner) updateJobProgress(progress, filesFound, filesProcessed int) {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	pfs.scanJob.Progress = progress
	pfs.scanJob.FilesFound = filesFound
	pfs.scanJob.FilesProcessed = filesProcessed
	pfs.db.Save(pfs.scanJob)
	
	// Only publish progress events for significant progress changes (every 10%)
	// or large file counts (every 100 files) to avoid overwhelming the event bus
	if progress%10 == 0 || filesProcessed%100 == 0 || filesProcessed == 1 {
		if pfs.eventBus != nil {
			progressEvent := events.NewSystemEvent(
				events.EventScanProgress,
				"Media Scan Progress",
				fmt.Sprintf("Scan #%d: %d%% complete, %d files processed", 
					pfs.jobID, progress, filesProcessed),
			)
			progressEvent.Data = map[string]interface{}{
				"libraryId":      pfs.scanJob.LibraryID,
				"scanJobId":      pfs.jobID,
				"progress":       progress,
				"filesFound":     filesFound,
				"filesProcessed": filesProcessed,
			}
			pfs.eventBus.PublishAsync(progressEvent)
		}
	}
}

// updateJobError updates the scan job with an error
func (pfs *ParallelFileScanner) updateJobError(errorMsg string) {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	pfs.scanJob.Status = "failed"
	pfs.scanJob.ErrorMessage = errorMsg
	now := time.Now()
	pfs.scanJob.CompletedAt = &now
	pfs.db.Save(pfs.scanJob)
}
