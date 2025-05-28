package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/metadata"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ParallelFileScanner implements a high-performance parallel file scanner
type ParallelFileScanner struct {
	db               *gorm.DB
	jobID            uint
	eventBus         events.EventBus
	scanJob          *database.ScanJob
	pathResolver     *utils.PathResolver
	progressEstimator *ProgressEstimator
	systemMonitor    *SystemLoadMonitor
	pluginManager    plugins.Manager
	
	// Worker management
	workerCount      int
	minWorkers       int              // Minimum number of workers
	maxWorkers       int              // Maximum number of workers
	activeWorkers    atomic.Int32     // Current number of active workers
	workerExitChan   chan int         // Channel for signaling workers to exit
	workQueue        chan scanWork
	resultQueue      chan *scanResult
	errorQueue       chan error
	
	// Directory walking - parallel implementation
	dirWorkerCount   int              // Number of directory walking workers
	dirQueue         chan dirWork     // Queue for directory scanning work
	activeDirWorkers atomic.Int32     // Current number of active directory workers
	
	// Batch processing
	batchSize        int
	batchTimeout     time.Duration
	batchProcessor   *BatchProcessor
	
	// State management
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	
	// Performance metrics
	filesProcessed   atomic.Int64
	bytesProcessed   atomic.Int64
	errorsCount      atomic.Int64
	filesSkipped     atomic.Int64  // Files that couldn't be processed
	startTime        time.Time
	
	// Cache for improved performance
	fileCache        *FileCache
	metadataCache    *MetadataCache
}

// scanWork represents a unit of work for scanning
type scanWork struct {
	path      string
	info      os.FileInfo
	libraryID uint
}

// dirWork represents a unit of work for directory scanning
type dirWork struct {
	path      string
	libraryID uint
}

// scanResult represents the result of scanning a file
type scanResult struct {
	mediaFile        *database.MediaFile
	musicMeta        *database.MusicMetadata
	path             string
	error            error
	needsPluginHooks bool
}

// BatchProcessor handles batch database operations
type BatchProcessor struct {
	db           *gorm.DB
	batchSize    int
	timeout      time.Duration
	mediaFiles   []*database.MediaFile
	musicMetas   []*database.MusicMetadata
	metaToPath   map[*database.MusicMetadata]string  // Track metadata to file path mapping
	mu           sync.Mutex
}

// FileCache provides fast lookups for existing files
type FileCache struct {
	cache map[string]*database.MediaFile
	mu    sync.RWMutex
}

// MetadataCache caches metadata extraction results
type MetadataCache struct {
	cache map[string]*database.MusicMetadata
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewParallelFileScanner creates a new parallel file scanner with optimizations
func NewParallelFileScanner(db *gorm.DB, jobID uint, eventBus events.EventBus, pluginManager plugins.Manager) *ParallelFileScanner {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Calculate optimal worker count based on CPU cores
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	} else if workerCount > 8 {
		workerCount = 8 // Cap at 8 workers to avoid overwhelming the system
	}
	
	// Set up adaptive worker pool parameters
	minWorkers := 2
	maxWorkers := runtime.NumCPU() * 2 // Allow up to 2x CPU cores for I/O bound operations
	if maxWorkers > 16 {
		maxWorkers = 16 // Cap at 16 workers
	}
	
	// Calculate directory worker count (fewer workers for directory scanning)
	dirWorkerCount := runtime.NumCPU() / 2
	if dirWorkerCount < 1 {
		dirWorkerCount = 1
	} else if dirWorkerCount > 4 {
		dirWorkerCount = 4 // Cap at 4 directory workers
	}
	
	scanner := &ParallelFileScanner{
		db:            db,
		jobID:         jobID,
		eventBus:      eventBus,
		pathResolver:  utils.NewPathResolver(),
		pluginManager: pluginManager,
		workerCount:   workerCount,
		minWorkers:    minWorkers,
		maxWorkers:    maxWorkers,
		dirWorkerCount: dirWorkerCount,
		workerExitChan: make(chan int, maxWorkers),
		workQueue:     make(chan scanWork, workerCount*100), // Buffered queue
		dirQueue:      make(chan dirWork, dirWorkerCount*500), // Buffered queue with larger buffer
		resultQueue:   make(chan *scanResult, workerCount*10),
		errorQueue:    make(chan error, workerCount),
		batchSize:     100, // Process files in batches of 100
		batchTimeout:  5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		fileCache:     &FileCache{
			cache: make(map[string]*database.MediaFile),
			mu:    sync.RWMutex{},
		},
	}
	
	// Initialize batch processor
	scanner.batchProcessor = &BatchProcessor{
		db:         db,
		batchSize:  scanner.batchSize,
		timeout:    scanner.batchTimeout,
		mediaFiles: make([]*database.MediaFile, 0, scanner.batchSize),
		musicMetas: make([]*database.MusicMetadata, 0, scanner.batchSize),
		metaToPath: make(map[*database.MusicMetadata]string),
	}
	
	// Initialize metadata cache
	scanner.metadataCache = &MetadataCache{
		cache: make(map[string]*database.MusicMetadata),
		mu:    sync.RWMutex{},
		ttl:   30 * time.Minute,
	}
	
	// Initialize progress estimator
	scanner.progressEstimator = NewProgressEstimator()
	
	// Initialize system load monitor
	scanner.systemMonitor = NewSystemLoadMonitor()
	
	return scanner
}

// Start begins the parallel scanning process
func (ps *ParallelFileScanner) Start(libraryID uint) error {
	ps.startTime = time.Now()
	
	// Load scan job
	if err := ps.loadScanJob(); err != nil {
		return err
	}
	
	// Update job status to running and set start time if not already set
	updates := map[string]interface{}{
		"status": string(utils.StatusRunning),
	}
	
	// Only set the start time if it's not already set (for resumed jobs)
	if ps.scanJob.StartedAt == nil {
		now := time.Now()
		updates["started_at"] = &now
		ps.scanJob.StartedAt = &now
	}
	
	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updates).Error; err != nil {
		fmt.Printf("Error updating job status to running: %v\n", err)
		return fmt.Errorf("failed to update job status to running: %w", err)
	}
	
	// Update in-memory status
	ps.scanJob.Status = string(utils.StatusRunning)
	fmt.Printf("Successfully updated job %d status to running\n", ps.jobID)
	
	// Pre-load existing files into cache for fast lookup
	if err := ps.preloadFileCache(libraryID); err != nil {
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
	}
	
	// Always re-count files for accurate progress tracking unless the job is already completed.
	// This ensures FilesFound is up-to-date for new, resumed, or previously crashed scans.
	if ps.scanJob.Status != string(utils.StatusCompleted) {
		fmt.Printf("Recounting files for job %d (status: %s)...\n", ps.jobID, ps.scanJob.Status)
		fileCount, err := ps.countFilesToScan(ps.scanJob.Library.Path)
		if err != nil {
			fmt.Printf("Warning: Failed to count files for job %d: %v\n", ps.jobID, err)
			// If counting fails, and we had a previous count, prefer that. Otherwise, estimator might have 0 total.
			if ps.scanJob.FilesFound > 0 {
				ps.progressEstimator.SetTotal(int64(ps.scanJob.FilesFound), 0)
				fmt.Printf("Using previous file count %d for job %d due to recount error.\n", ps.scanJob.FilesFound, ps.jobID)
			} else {
				// No previous count and recount failed, estimator will start with 0 total until files are found.
				// This is not ideal but better than crashing.
				ps.progressEstimator.SetTotal(0, 0)
				fmt.Printf("Warning: No previous file count and recount failed for job %d. Progress may be inaccurate initially.\n", ps.jobID)
			}
		} else {
			fmt.Printf("Job %d: Recounted %d files. Previous count: %d.\n", ps.jobID, fileCount, ps.scanJob.FilesFound)
			if fileCount != ps.scanJob.FilesFound {
				fmt.Printf("Job %d: Updating FilesFound in DB from %d to %d.\n", ps.jobID, ps.scanJob.FilesFound, fileCount)
				ps.scanJob.FilesFound = fileCount // Update in-memory job object
				if errDb := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).
					Update("files_found", fileCount).Error; errDb != nil {
					fmt.Printf("Failed to update files_found in database for job %d: %v\n", ps.jobID, errDb)
				}
			} else {
				fmt.Printf("Job %d: File count %d remains unchanged.\n", ps.jobID, fileCount)
			}
			ps.progressEstimator.SetTotal(int64(fileCount), 0)
		}
	} else {
		// Job is completed, just set the estimator total from the existing FilesFound.
		ps.progressEstimator.SetTotal(int64(ps.scanJob.FilesFound), 0)
		fmt.Printf("Job %d is completed. Using existing FilesFound: %d for progress estimator total.\n", ps.jobID, ps.scanJob.FilesFound)
	}

	// Load existing progress from database (for resumed/restarted scans)
	// This should happen *after* FilesFound is potentially re-counted and estimator total is set.
	// We check FilesProcessed > 0 to see if there's any progress to load.
	// For a brand new scan that was never paused/crashed, FilesProcessed would be 0.
	if ps.scanJob.FilesProcessed > 0 && ps.scanJob.Status != string(utils.StatusCompleted) {
		currentFilesProcessed := int64(ps.scanJob.FilesProcessed)
		currentBytesProcessed := ps.scanJob.BytesProcessed

		// Ensure FilesProcessed does not exceed the (potentially new) FilesFound
		if currentFilesProcessed > int64(ps.scanJob.FilesFound) && ps.scanJob.FilesFound > 0 {
			fmt.Printf("Warning: Job %d - FilesProcessed (%d) from DB exceeds new FilesFound (%d). Resetting FilesProcessed to 0 for resume.\n",
				ps.jobID, currentFilesProcessed, ps.scanJob.FilesFound)
			currentFilesProcessed = 0 // Reset to avoid starting with > 100% progress
			currentBytesProcessed = 0
			// Optionally, update DB here if we want this reset to be persistent immediately
			// For now, it will be corrected on the next progress update.
		}
		
		ps.filesProcessed.Store(currentFilesProcessed)
		ps.bytesProcessed.Store(currentBytesProcessed)
		
		ps.progressEstimator.Update(currentFilesProcessed, currentBytesProcessed)
		fmt.Printf("Resuming/restarting scan job %d with %d/%d files processed (from DB).\n",
			ps.jobID, currentFilesProcessed, ps.scanJob.FilesFound)
	} else if ps.scanJob.Status == string(utils.StatusCompleted) {
		// For completed jobs, ensure the atomic counters reflect the final DB state.
		ps.filesProcessed.Store(int64(ps.scanJob.FilesProcessed))
		ps.bytesProcessed.Store(ps.scanJob.BytesProcessed)
		ps.progressEstimator.Update(int64(ps.scanJob.FilesProcessed), ps.scanJob.BytesProcessed) // Ensure estimator reflects final state
		fmt.Printf("Job %d is completed. Initializing counters to final values: %d files, %d bytes.\n", ps.jobID, ps.scanJob.FilesProcessed, ps.scanJob.BytesProcessed)
	} else {
		// New scan or no prior progress, counters start at 0. Estimator total is set, current is 0.
		ps.filesProcessed.Store(0)
		ps.bytesProcessed.Store(0)
		ps.progressEstimator.Update(0,0) // Start fresh
		fmt.Printf("Starting new scan or scan with no prior progress for job %d. FilesFound: %d.\n", ps.jobID, ps.scanJob.FilesFound)
	}
	
	// Start initial workers (start with minimum workers)
	for i := 0; i < ps.minWorkers; i++ {
		ps.wg.Add(1)
		go ps.worker(i)
	}
	
	// Start directory workers for parallel directory walking
	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
	}
	
	// Start directory scanner BEFORE the queue manager to ensure work is added first
	ps.wg.Add(1)
	go ps.scanDirectory(libraryID)
	
	// Start directory queue manager
	ps.wg.Add(1)
	go ps.dirQueueManager()
	
	// Start work queue closer that waits for all directory scanning to complete
	ps.wg.Add(1)
	go ps.workQueueCloser()
	
	// Start worker pool manager for adaptive scaling
	ps.wg.Add(1)
	go ps.workerPoolManager()
	
	// Start result processor
	ps.wg.Add(1)
	go ps.resultProcessor()
	
	// Start batch flush timer
	ps.wg.Add(1)
	go ps.batchFlusher()
	
	// Wait for completion
	ps.wg.Wait()
	
	// Clean up channels safely
	defer func() {
		// Close channels in the right order to prevent panics
		select {
		case <-ps.resultQueue:
		default:
			close(ps.resultQueue)
		}
		
		select {
		case <-ps.errorQueue:
		default:
			close(ps.errorQueue)
		}
	}()
	
	// Check if the scan was canceled/paused
	select {
	case <-ps.ctx.Done():
		// Context was canceled, which means the scan was paused
		// We don't need to do anything as the Pause() method has already updated the status
		fmt.Printf("Scan job %d was paused, not updating to completed status\n", ps.jobID)
		return nil
	default:
		// Context was not canceled, scan completed normally
		// Final batch flush
		if err := ps.batchProcessor.Flush(); err != nil {
			fmt.Printf("Error flushing final batch: %v\n", err)
		}
		
		// Update final job status
		ps.updateFinalStatus()
		
		return nil
	}
}

// Resume resumes a previously paused scan
func (ps *ParallelFileScanner) Resume(libraryID uint) error {
	ps.startTime = time.Now()
	
	// Reset the context to ensure we have a fresh context for the resumed scan
	ps.ctx, ps.cancel = context.WithCancel(context.Background())
	
	// Reinitialize channels for resumed scan (they may have been closed during pause)
	ps.workQueue = make(chan scanWork, ps.workerCount*100)
	ps.dirQueue = make(chan dirWork, ps.dirWorkerCount*500)
	ps.resultQueue = make(chan *scanResult, ps.workerCount*10)
	ps.errorQueue = make(chan error, ps.workerCount)
	ps.workerExitChan = make(chan int, ps.maxWorkers)
	
	// Load the scan job to get current status
	if err := ps.loadScanJob(); err != nil {
		return fmt.Errorf("failed to load scan job for resume: %w", err)
	}
	
	// Update job status to running (for resumed jobs)
	updates := map[string]interface{}{
		"status": string(utils.StatusRunning),
		"error_message": "", // Clear any previous error message when resuming
	}
	
	// Only set the start time if it's not already set (preserve original start time)
	if ps.scanJob.StartedAt == nil {
		now := time.Now()
		updates["started_at"] = &now
		ps.scanJob.StartedAt = &now
	}
	
	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updates).Error; err != nil {
		fmt.Printf("Error updating job status to running: %v\n", err)
		return fmt.Errorf("failed to update job status to running: %w", err)
	}
	
	// Update in-memory status
	ps.scanJob.Status = string(utils.StatusRunning)
	ps.scanJob.ErrorMessage = "" // Clear error message in memory as well
	fmt.Printf("Successfully updated job %d status to running (resumed)\n", ps.jobID)
	
	// Pre-load existing files into cache for fast lookup
	if err := ps.preloadFileCache(libraryID); err != nil {
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
	}
	
	// Reset progress estimator for accurate ETA calculation
	ps.progressEstimator = NewProgressEstimator()
	
	// Set the total files count from the database (don't recount for resumed scans)
	if ps.scanJob.FilesFound > 0 {
		ps.progressEstimator.SetTotal(int64(ps.scanJob.FilesFound), 0)
		
		// Load existing progress from database
		ps.filesProcessed.Store(int64(ps.scanJob.FilesProcessed))
		ps.bytesProcessed.Store(ps.scanJob.BytesProcessed)
		
		// Update the progress estimator with current progress
		filesProcessed := ps.filesProcessed.Load()
		bytesProcessed := ps.bytesProcessed.Load()
		ps.progressEstimator.Update(filesProcessed, bytesProcessed)
		
		fmt.Printf("Resuming scan job %d with %d/%d files already processed\n", 
			ps.jobID, filesProcessed, ps.scanJob.FilesFound)
	} else {
		fmt.Printf("Warning: Resuming scan job %d but FilesFound is 0, will need to count files\n", 
			ps.jobID)
		
		// If no files found yet, we need to count them first
		fileCount, err := ps.countFilesToScan(ps.scanJob.Library.Path)
		if err != nil {
			fmt.Printf("Warning: Failed to count files: %v\n", err)
		} else {
			// Update the database with the file count
			ps.scanJob.FilesFound = fileCount
			if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).
				Update("files_found", fileCount).Error; err != nil {
				fmt.Printf("Failed to update files_found in database: %v\n", err)
			}
			ps.progressEstimator.SetTotal(int64(fileCount), 0)
		}
	}
	
	// Start initial workers (start with minimum workers)
	for i := 0; i < ps.minWorkers; i++ {
		ps.wg.Add(1)
		go ps.worker(i)
	}
	
	// Start directory workers for parallel directory walking
	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
	}
	
	// Start directory scanner BEFORE the queue manager to ensure work is added first
	ps.wg.Add(1)
	go ps.scanDirectory(libraryID)
	
	// Start directory queue manager
	ps.wg.Add(1)
	go ps.dirQueueManager()
	
	// Start work queue closer that waits for all directory scanning to complete
	ps.wg.Add(1)
	go ps.workQueueCloser()
	
	// Start worker pool manager for adaptive scaling
	ps.wg.Add(1)
	go ps.workerPoolManager()
	
	// Start result processor
	ps.wg.Add(1)
	go ps.resultProcessor()
	
	// Start batch flush timer
	ps.wg.Add(1)
	go ps.batchFlusher()
	
	// Wait for completion
	ps.wg.Wait()
	
	// Clean up channels safely
	defer func() {
		// Close channels in the right order to prevent panics
		select {
		case <-ps.resultQueue:
		default:
			close(ps.resultQueue)
		}
		
		select {
		case <-ps.errorQueue:
		default:
			close(ps.errorQueue)
		}
	}()
	
	// Check if the scan was canceled/paused
	select {
	case <-ps.ctx.Done():
		// Context was canceled, which means the scan was paused
		// We don't need to do anything as the Pause() method has already updated the status
		fmt.Printf("Scan job %d was paused, not updating to completed status\n", ps.jobID)
		return nil
	default:
		// Context was not canceled, scan completed normally
		// Final batch flush
		if err := ps.batchProcessor.Flush(); err != nil {
			fmt.Printf("Error flushing final batch: %v\n", err)
		}
		
		// Update final job status
		ps.updateFinalStatus()
		
		return nil
	}
}

// loadScanJob loads the scan job from the database
func (ps *ParallelFileScanner) loadScanJob() error {
	if err := ps.db.Preload("Library").First(&ps.scanJob, ps.jobID).Error; err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}
	return nil
}

// updateFinalStatus updates the final status of the scan job
func (ps *ParallelFileScanner) updateFinalStatus() {
	// If the job was already marked as paused, don't change its status
	var currentJob database.ScanJob
	if err := ps.db.First(&currentJob, ps.jobID).Error; err == nil {
		if currentJob.Status == "paused" {
			fmt.Printf("Job %d is paused, not updating status to completed\n", ps.jobID)
			return
		}
	}
	
	filesProcessed := ps.filesProcessed.Load()
	bytesProcessed := ps.bytesProcessed.Load()
	errorsCount := ps.errorsCount.Load()
	filesSkipped := ps.filesSkipped.Load()
	
	// Calculate completion statistics
	totalFilesFound := int64(ps.scanJob.FilesFound)
	filesNotProcessed := totalFilesFound - filesProcessed

	// If more files were processed than initially found (e.g., files added mid-scan),
	// update totalFilesFound to reflect the actual number of files handled.
	if filesProcessed > totalFilesFound {
		fmt.Printf("Job ID %d: Adjusting FilesFound from %d to %d as more files were processed.\n", ps.jobID, totalFilesFound, filesProcessed)
		totalFilesFound = filesProcessed
		filesNotProcessed = 0 // Since all processed files are now considered 'found'
		ps.scanJob.FilesFound = int(totalFilesFound) // Update the job object as well for DB save
	}
	
	// Log comprehensive scan summary
	fmt.Printf("\n=== SCAN COMPLETED ===\n")
	fmt.Printf("Job ID: %d\n", ps.jobID)
	fmt.Printf("Files found: %d\n", totalFilesFound)
	fmt.Printf("Files processed: %d\n", filesProcessed)
	fmt.Printf("Files skipped (metadata errors): %d\n", filesSkipped)
	fmt.Printf("Files not processed: %d\n", filesNotProcessed)
	fmt.Printf("Processing errors: %d\n", errorsCount)
	fmt.Printf("Data processed: %.2f GB\n", float64(bytesProcessed)/(1024*1024*1024))
	fmt.Printf("Success rate: %.1f%%\n", (float64(filesProcessed)/float64(totalFilesFound))*100)
	fmt.Printf("======================\n\n")
	
	// Update scan job with final stats
	now := time.Now()
	ps.scanJob.FilesProcessed = int(filesProcessed)
	ps.scanJob.BytesProcessed = bytesProcessed
	ps.scanJob.Progress = 100  // Always mark as 100% when scan completes
	ps.scanJob.CompletedAt = &now
	ps.scanJob.UpdatedAt = now
	
	// Always mark as completed, but include summary in error message if there were issues
	ps.scanJob.Status = "completed"
	
	if filesNotProcessed > 0 || errorsCount > 0 || filesSkipped > 0 {
		ps.scanJob.ErrorMessage = fmt.Sprintf("Processed %d/%d files. Skipped: %d (metadata errors), Errors: %d", 
			filesProcessed, totalFilesFound, filesSkipped, errorsCount)
	} else {
		ps.scanJob.ErrorMessage = ""
	}
	
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		fmt.Printf("Failed to update final scan job status: %v\n", err)
	} else {
		// Publish completion event if DB save was successful
		if ps.eventBus != nil {
			completionEvent := events.NewSystemEvent(
				events.EventScanCompleted,
				"Scan Job Completed",
				fmt.Sprintf("Scan job %d completed. Processed %d/%d files. Errors: %d, Skipped: %d", 
					ps.jobID, filesProcessed, totalFilesFound, errorsCount, filesSkipped),
			)
			completionEvent.Data = map[string]interface{}{
				"jobId":          ps.jobID,
				"libraryId":      ps.scanJob.LibraryID,
				"status":         ps.scanJob.Status, // Should be "completed"
				"filesFound":     totalFilesFound,
				"filesProcessed": filesProcessed,
				"bytesProcessed": bytesProcessed,
				"errorsCount":    errorsCount,
				"filesSkipped":   filesSkipped,
				"progress":       ps.scanJob.Progress, // Should be 100
				"errorMessage":   ps.scanJob.ErrorMessage,
				"completedAt":    ps.scanJob.CompletedAt.Format(time.RFC3339),
			}
			ps.eventBus.PublishAsync(completionEvent)
		}
	}
}

// preloadFileCache loads existing files into memory for fast lookup
func (ps *ParallelFileScanner) preloadFileCache(libraryID uint) error {
	var existingFiles []database.MediaFile
	
	// Load files in chunks to avoid memory issues
	chunkSize := 1000
	offset := 0
	
	for {
		var chunk []database.MediaFile
		err := ps.db.Where("library_id = ?", libraryID).
			Offset(offset).
			Limit(chunkSize).
			Find(&chunk).Error
		
		if err != nil {
			return err
		}
		
		if len(chunk) == 0 {
			break
		}
		
		// Add to cache
		ps.fileCache.mu.Lock()
		for _, file := range chunk {
			ps.fileCache.cache[file.Path] = &file
		}
		ps.fileCache.mu.Unlock()
		
		existingFiles = append(existingFiles, chunk...)
		offset += chunkSize
		
		// If we got fewer results than requested, we've reached the end
		if len(chunk) < chunkSize {
			break
		}
	}
	
	fmt.Printf("Preloaded %d existing files into cache\n", len(existingFiles))
	return nil
}

// countFilesToScan counts the total number of media files in the library
// This is used to provide accurate progress tracking
func (ps *ParallelFileScanner) countFilesToScan(libraryPath string) (int, error) {
	basePath, err := ps.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve directory: %w", err)
	}
	
	// Use atomic counter to count files safely
	var totalCount atomic.Int64
	
	// Create a separate context for counting so it can be canceled independently
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Use filepath.WalkDir for efficient directory traversal
	err = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		// Check if context was canceled (timeout or external cancellation)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err != nil {
			fmt.Printf("Warning: Error accessing path %s: %v\n", path, err)
			return nil // Continue despite errors
		}
		
		// Skip directories
		if d.IsDir() {
			return nil
		}
		
		// Only count files that will actually be processed
		// For media libraries, include both audio and video files
		if metadata.IsMediaLibraryFile(path) {
			totalCount.Add(1)
			
			// Log progress periodically
			count := totalCount.Load()
			if count%1000 == 0 {
				fmt.Printf("Counted %d files so far...\n", count)
			}
		}
		
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("error counting files: %w", err)
	}
	
	fileCount := int(totalCount.Load())
	fmt.Printf("Found a total of %d media files to scan\n", fileCount)
	return fileCount, nil
}

// scanDirectory initiates parallel directory walking
func (ps *ParallelFileScanner) scanDirectory(libraryID uint) {
	defer ps.wg.Done()
	
	libraryPath := ps.scanJob.Library.Path
	fmt.Printf("DEBUG: scanDirectory starting for library %d with path: %s\n", libraryID, libraryPath)
	
	basePath, err := ps.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to resolve directory %s: %v\n", libraryPath, err)
		ps.errorQueue <- fmt.Errorf("failed to resolve directory: %w", err)
		return
	}
	
	fmt.Printf("DEBUG: Resolved path from %s to %s\n", libraryPath, basePath)
	
	// Check if the path exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		fmt.Printf("ERROR: Directory does not exist: %s\n", basePath)
		ps.errorQueue <- fmt.Errorf("directory does not exist: %s", basePath)
		return
	}
	
	fmt.Printf("DEBUG: Adding root directory to queue: %s\n", basePath)
	
	// Start with the root directory in the queue
	// Use a timeout to ensure we don't block indefinitely
	rootWork := dirWork{
		path:      basePath,
		libraryID: libraryID,
	}
	
	select {
	case ps.dirQueue <- rootWork:
		fmt.Printf("DEBUG: Successfully added root directory to queue: %s\n", basePath)
	case <-time.After(10 * time.Second):
		fmt.Printf("ERROR: Timeout adding root directory to queue: %s\n", basePath)
		ps.errorQueue <- fmt.Errorf("timeout adding root directory to queue: %s", basePath)
		return
	case <-ps.ctx.Done():
		fmt.Printf("DEBUG: scanDirectory cancelled before adding root directory\n")
		return
	}
	
	// This method just initiates the scanning; the dirQueueManager will handle closing
	fmt.Printf("DEBUG: scanDirectory completed for library %d\n", libraryID)
}

// worker processes files from the work queue
func (ps *ParallelFileScanner) worker(id int) {
	defer ps.wg.Done()
	defer ps.activeWorkers.Add(-1) // Decrement active worker count when exiting
	
	ps.activeWorkers.Add(1) // Increment active worker count
	fmt.Printf("DEBUG: Worker %d started\n", id)
	
	for {
		select {
		case work, ok := <-ps.workQueue:
			if !ok {
				// Work queue closed, exit
				fmt.Printf("DEBUG: Worker %d exiting - work queue closed\n", id)
				return
			}
			
			fmt.Printf("DEBUG: Worker %d processing file: %s\n", id, work.path)
			
			// Check for cancellation
			select {
			case <-ps.ctx.Done():
				fmt.Printf("DEBUG: Worker %d exiting - context canceled\n", id)
				return
			default:
			}
			
			result := ps.processFile(work)
			
			// Safely send result to avoid panic on closed channel
			select {
			case ps.resultQueue <- result:
				fmt.Printf("DEBUG: Worker %d sent result for: %s\n", id, work.path)
			case <-ps.ctx.Done():
				fmt.Printf("DEBUG: Worker %d exiting - context canceled while sending result\n", id)
				return
			default:
				// Channel might be closed or full, check context and exit gracefully
				select {
				case <-ps.ctx.Done():
					fmt.Printf("DEBUG: Worker %d exiting - context canceled, result queue unavailable\n", id)
					return
				default:
					// Try one more time with a timeout
					select {
					case ps.resultQueue <- result:
						fmt.Printf("DEBUG: Worker %d sent result for: %s (retry)\n", id, work.path)
					case <-time.After(100 * time.Millisecond):
						fmt.Printf("WARNING: Worker %d timeout sending result for: %s\n", id, work.path)
					case <-ps.ctx.Done():
						fmt.Printf("DEBUG: Worker %d exiting - context canceled during retry\n", id)
						return
					}
				}
			}
			
		case exitID := <-ps.workerExitChan:
			if exitID == id {
				// This worker was signaled to exit
				fmt.Printf("DEBUG: Worker %d exiting - received exit signal\n", id)
				return
			}
			// Put the signal back if it's not for this worker
			ps.workerExitChan <- exitID
			
		case <-ps.ctx.Done():
			fmt.Printf("DEBUG: Worker %d exiting - context canceled\n", id)
			return
		}
	}
}

// processFile processes a single file
func (ps *ParallelFileScanner) processFile(work scanWork) *scanResult {
	result := &scanResult{path: work.path}
	
	// Check cache first
	ps.fileCache.mu.RLock()
	cachedFile, exists := ps.fileCache.cache[work.path]
	ps.fileCache.mu.RUnlock()
	
	if exists && cachedFile.Size == work.info.Size() {
		// File hasn't changed, but we still need to call plugin hooks if it's a music file
		// Note: We count files here and return early, so we don't double-count later
		ps.filesProcessed.Add(1)
		ps.bytesProcessed.Add(work.info.Size())
		
		// Check if this is a music file and if we should call plugin hooks
		if metadata.IsMusicFile(work.path) {
			// Try to get existing metadata for this file
			var existingMeta database.MusicMetadata
			if err := ps.db.Where("media_file_id = ?", cachedFile.ID).First(&existingMeta).Error; err == nil {
				// Found existing metadata, set up result for plugin hooks
				result.mediaFile = cachedFile
				result.musicMeta = &existingMeta
				result.needsPluginHooks = true
				fmt.Printf("[DEBUG][processFile] Cached file with metadata, will call plugin hooks: %s\n", work.path)
			} else {
				fmt.Printf("[DEBUG][processFile] Cached file but no metadata found: %s\n", work.path)
			}
		}
		
		return result
	}
	
	// Calculate file hash (use a faster algorithm for large files)
	hash, err := ps.calculateFileHashOptimized(work.path, work.info.Size())
	if err != nil {
		result.error = err
		return result
	}
	
	// Create or update media file
	mediaFile := &database.MediaFile{
		Path:      work.path,
		Size:      work.info.Size(),
		Hash:      hash,
		LibraryID: work.libraryID,
		LastSeen:  time.Now(),
	}
	
	// If file exists in cache, update the ID
	if cachedFile != nil {
		mediaFile.ID = cachedFile.ID
	}
	
	result.mediaFile = mediaFile
	
	// Extract metadata for music files (audio files only)
	isMusicFile := metadata.IsMusicFile(work.path)
	fmt.Printf("[DEBUG][processFile] Processing %s, isMusicFile=%t\n", work.path, isMusicFile)
	if isMusicFile {
		// Check metadata cache
		ps.metadataCache.mu.RLock()
		cachedMeta, metaExists := ps.metadataCache.cache[hash]
		ps.metadataCache.mu.RUnlock()
		
		if !metaExists {
			fmt.Printf("[DEBUG][processFile] Extracting metadata for %s\n", work.path)
			musicMeta, err := metadata.ExtractMusicMetadata(work.path, mediaFile)
			if err != nil {
				fmt.Printf("[ERROR][processFile] Failed to extract metadata from %s: %v\n", work.path, err)
				ps.filesSkipped.Add(1)
				// Still save the media file record, just without metadata
			} else {
				result.musicMeta = musicMeta
				fmt.Printf("[DEBUG][processFile] Successfully extracted metadata for %s\n", work.path)
				
				// Cache the metadata
				ps.metadataCache.mu.Lock()
				ps.metadataCache.cache[hash] = musicMeta
				ps.metadataCache.mu.Unlock()
				
				// Mark that this file needs plugin hook processing
				result.needsPluginHooks = true
			}
		} else {
			fmt.Printf("[DEBUG][processFile] Using cached metadata for %s\n", work.path)
			result.musicMeta = cachedMeta
			// Mark that this file needs plugin hook processing even when using cached metadata
			result.needsPluginHooks = true
		}
	} else {
		// Video files - just save the media file record without music metadata
		fmt.Printf("[DEBUG][processFile] Video file detected: %s\n", work.path)
	}
	
	// Update metrics (only for files that weren't found in cache)
	ps.filesProcessed.Add(1)
	ps.bytesProcessed.Add(work.info.Size())
	
	return result
}

// calculateFileHashOptimized calculates file hash with optimizations for large files
func (ps *ParallelFileScanner) calculateFileHashOptimized(path string, size int64) (string, error) {
	// For small files, hash the entire file
	if size < 10*1024*1024 { // 10MB
		return utils.CalculateFileHash(path)
	}
	
	// For large files, use a sampling approach
	// Hash the first 1MB, middle 1MB, and last 1MB
	return utils.CalculateFileHashSampled(path, size)
}

// callPluginHooks calls scanner hook plugins for a processed media file
func (ps *ParallelFileScanner) callPluginHooks(mediaFile *database.MediaFile, musicMeta *database.MusicMetadata) {
	if ps.pluginManager == nil {
		fmt.Printf("[DEBUG][callPluginHooks] Plugin manager is nil\n")
		return
	}
	
	// Get scanner hook clients
	scannerHookClients := ps.pluginManager.GetScannerHooks()
	fmt.Printf("[DEBUG][callPluginHooks] Found %d scanner hook clients\n", len(scannerHookClients))
	if len(scannerHookClients) == 0 {
		return
	}
	
	// Convert metadata to map[string]string for plugin interface
	metadataMap := map[string]string{
		"title":      musicMeta.Title,
		"artist":     musicMeta.Artist,
		"album":      musicMeta.Album,
		"year":       fmt.Sprintf("%d", musicMeta.Year),
		"track":      fmt.Sprintf("%d", musicMeta.Track),
		"genre":      musicMeta.Genre,
		"duration":   fmt.Sprintf("%f", musicMeta.Duration),
		"hasArtwork": fmt.Sprintf("%t", musicMeta.HasArtwork),
	}
	
	// Call each scanner hook plugin
	ctx := context.Background()
	for _, client := range scannerHookClients {
		req := &proto.OnMediaFileScannedRequest{
			MediaFileId: uint32(mediaFile.ID),
			FilePath:    mediaFile.Path,
			Metadata:    metadataMap,
		}
		
		_, err := client.OnMediaFileScanned(ctx, req)
		if err != nil {
			fmt.Printf("WARNING: plugin scanner hook OnMediaFileScanned failed for file %s: %v\n", mediaFile.Path, err)
		} else {
			fmt.Printf("DEBUG: Successfully called plugin hook for file %s\n", mediaFile.Path)
		}
	}
}

// resultProcessor handles results from workers
func (ps *ParallelFileScanner) resultProcessor() {
	defer ps.wg.Done()
	// Don't close resultQueue here - let the main cleanup handle it
	// defer close(ps.resultQueue)
	
	updateTicker := time.NewTicker(1 * time.Second)
	defer updateTicker.Stop()
	
	for {
		select {
		case result, ok := <-ps.resultQueue:
			if !ok {
				return
			}
			
			if result.error != nil {
				ps.errorsCount.Add(1)
				continue
			}
			
					// Add to batch
		if result.mediaFile != nil {
			ps.batchProcessor.AddMediaFile(result.mediaFile)
		}
		if result.musicMeta != nil {
			ps.batchProcessor.AddMusicMetadataWithPath(result.musicMeta, result.path)
		}
		
		// Call plugin hooks if needed (after metadata is extracted)
		if result.needsPluginHooks {
			fmt.Printf("[DEBUG][resultProcessor] needsPluginHooks=true for file: %s\n", result.path)
			if result.musicMeta != nil && result.mediaFile != nil {
				fmt.Printf("[DEBUG][resultProcessor] Calling plugin hooks for: %s\n", result.path)
				ps.callPluginHooks(result.mediaFile, result.musicMeta)
			} else {
				fmt.Printf("[DEBUG][resultProcessor] Skipping plugin hooks - musicMeta=%v, mediaFile=%v\n", result.musicMeta != nil, result.mediaFile != nil)
			}
		} else {
			fmt.Printf("[DEBUG][resultProcessor] needsPluginHooks=false for file: %s\n", result.path)
		}
			
		case <-updateTicker.C:
			// Update progress periodically
			ps.updateProgress()
			
		case <-ps.ctx.Done():
			return
		}
	}
}

// batchFlusher periodically flushes batches to the database
func (ps *ParallelFileScanner) batchFlusher() {
	defer ps.wg.Done()
	
	ticker := time.NewTicker(ps.batchTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := ps.batchProcessor.FlushIfNeeded(); err != nil {
				fmt.Printf("Error flushing batch: %v\n", err)
			}
			
		case <-ps.ctx.Done():
			return
		}
	}
}

// updateProgress updates the scan job progress
func (ps *ParallelFileScanner) updateProgress() {
	filesProcessed := ps.filesProcessed.Load()
	bytesProcessed := ps.bytesProcessed.Load()
	errorsCount := ps.errorsCount.Load()
	filesSkipped := ps.filesSkipped.Load()
	_ = filesSkipped // Prevent unused variable error

	// Debug logging to help track progress updates
	// fmt.Printf("DEBUG: updateProgress called - files: %d, bytes: %d, skipped: %d, errors: %d, queue: %d, active workers: %d\n",
	// filesProcessed, bytesProcessed, filesSkipped, errorsCount, len(ps.workQueue), ps.activeWorkers.Load())

	// Update progress estimator
	ps.progressEstimator.Update(filesProcessed, bytesProcessed)
	
	// Calculate progress percentage
	progress := 0
	if ps.scanJob.FilesFound > 0 {
		// Make sure we don't exceed 100% even if more files were added during the scan
		progress = int((float64(filesProcessed) / float64(ps.scanJob.FilesFound)) * 100)
		if progress > 100 {
			progress = 100
		}
		// Log progress for debugging
		if filesProcessed % 10 == 0 || filesProcessed < 10 {  // Log more frequently for debugging
			fmt.Printf("Progress update: %d/%d files processed (%d%%)\n", 
				filesProcessed, ps.scanJob.FilesFound, progress)
		}
	}
	
	// First, get the most recent status from the database to avoid overwriting a pause
	var currentStatus string
	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Select("status").Scan(&currentStatus).Error; err == nil {
		// If the job is paused in the database, respect that status
		if currentStatus == "paused" {
			ps.scanJob.Status = "paused"
			// Only update the progress metrics but preserve the paused status
			updates := map[string]interface{}{
				"files_processed": int(filesProcessed),
				"bytes_processed": bytesProcessed,
				"progress":       progress,
				"updated_at":     time.Now(),
			}
			if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updates).Error; err != nil {
				fmt.Printf("Failed to update scan job progress: %v\n", err)
			}
			return
		}
	}
	
	// Update scan job fields
	updates := map[string]interface{}{
		"files_processed": int(filesProcessed),
		"bytes_processed": bytesProcessed,
		"progress":       progress,
		"updated_at":     time.Now(),
	}
	
	// Update the status if needed
	if ps.scanJob.Status != "running" {
		// If we're making progress, ensure the status is set to running
		updates["status"] = "running"
		ps.scanJob.Status = "running"
		
		// Update the start time if it's not already set
		now := time.Now()
		if ps.scanJob.StartedAt == nil {
			updates["started_at"] = &now
			ps.scanJob.StartedAt = &now
		}
		
		fmt.Printf("Updating job %d status to running\n", ps.jobID)
	}
	
	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updates).Error; err != nil {
		fmt.Printf("Failed to update scan job progress: %v\n", err)
	}
	
	// Publish progress event (publish more frequently for debugging)
	if ps.eventBus != nil && (filesProcessed%10 == 0 || filesProcessed < 10) {
		event := events.NewSystemEvent(
			events.EventScanProgress,
			"Scan Progress Update",
			fmt.Sprintf("Processed %d files (%.2f GB) - Workers: %d active", 
				filesProcessed, 
				float64(bytesProcessed)/(1024*1024*1024),
				ps.activeWorkers.Load()),
		)
		event.Data = map[string]interface{}{
			"jobId":          ps.jobID,
			"filesProcessed": filesProcessed,
			"bytesProcessed": bytesProcessed,
			"filesFound":     ps.scanJob.FilesFound,
			"errorsCount":    errorsCount,
			"progress":       progress,
			"activeWorkers":  ps.activeWorkers.Load(),
			"minWorkers":     ps.minWorkers,
			"maxWorkers":     ps.maxWorkers,
			"queueDepth":     len(ps.workQueue),
		}
		ps.eventBus.PublishAsync(event)
	}
}

// Pause pauses the scanner
func (ps *ParallelFileScanner) Pause() {
	// Mark the job as paused in memory before canceling
	ps.scanJob.Status = "paused"
	
	// Save the current progress state to database
	updates := map[string]interface{}{
		"status":         "paused",
		"files_processed": int(ps.filesProcessed.Load()),
		"bytes_processed": ps.bytesProcessed.Load(),
		"progress":       ps.scanJob.Progress,
		"updated_at":     time.Now(),
	}
	
	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updates).Error; err != nil {
		fmt.Printf("Failed to update scan job as paused: %v\n", err)
	}
	
	// Publish pause event
	if ps.eventBus != nil {
		pausedEvent := events.NewSystemEvent(
			events.EventScanPaused,
			"Scan Job Paused",
			fmt.Sprintf("Scan job %d has been paused.", ps.jobID),
		)
		pausedEvent.Data = map[string]interface{}{
			"jobId":          ps.jobID,
			"libraryId":      ps.scanJob.LibraryID,
			"status":         "paused",
			"filesProcessed": ps.filesProcessed.Load(), // Current progress at time of pause
			"bytesProcessed": ps.bytesProcessed.Load(),
			"progress":       ps.scanJob.Progress,      // Progress percentage from job object
		}
		ps.eventBus.PublishAsync(pausedEvent)
	}

	// Cancel all operations
	ps.cancel()
}

// Adaptive Worker Pool Management

// adjustWorkers dynamically adjusts the number of workers based on queue depth and system load
func (ps *ParallelFileScanner) adjustWorkers() {
	queueLen := len(ps.workQueue)
	currentWorkers := int(ps.activeWorkers.Load())
	
	// Get system load metrics
	cpuUsage, memUsage, _ := ps.systemMonitor.GetMetrics()
	loadScore := ps.systemMonitor.GetLoadScore()
	
	// Calculate target workers based on queue depth and system load
	targetWorkers := currentWorkers
	
	// Scale up if queue is getting backed up and system can handle more load
	if queueLen > currentWorkers && currentWorkers < ps.maxWorkers && loadScore < 70 {
		// Scale more aggressively if queue is very full and system load is low
		if queueLen > currentWorkers*3 && loadScore < 40 {
			// Add up to 2 workers at once if load is very low and queue is very backed up
			targetWorkers = currentWorkers + 2
		} else {
			targetWorkers = currentWorkers + 1
		}
		
		// Respect max workers limit
		if targetWorkers > ps.maxWorkers {
			targetWorkers = ps.maxWorkers
		}
	}
	
	// Scale down under various conditions
	if currentWorkers > ps.minWorkers {
		// Scale down if queue is empty
		if queueLen == 0 {
			targetWorkers = currentWorkers - 1
		}
		
		// Scale down if system is overloaded
		if cpuUsage > 90 || memUsage > 90 {
			targetWorkers = currentWorkers - 1
			fmt.Printf("Scaling down due to high system load: CPU: %.1f%%, Mem: %.1f%%\n",
				cpuUsage, memUsage)
		}
		
		// Ensure we respect minimum workers limit
		if targetWorkers < ps.minWorkers {
			targetWorkers = ps.minWorkers
		}
	}
	
	// Apply worker adjustments
	if targetWorkers > currentWorkers {
		// Add workers
		for i := currentWorkers; i < targetWorkers; i++ {
			ps.wg.Add(1)
			go ps.worker(i)
		}
		fmt.Printf("Scaled up workers: %d -> %d (queue depth: %d)\n", currentWorkers, targetWorkers, queueLen)
	} else if targetWorkers < currentWorkers {
		// Remove workers
		workersToRemove := currentWorkers - targetWorkers
		for i := 0; i < workersToRemove; i++ {
			// Signal workers to exit by sending their IDs
			// We'll signal the highest numbered workers first
			select {
			case ps.workerExitChan <- currentWorkers - 1 - i:
			default:
				// Channel full, skip this worker for now
			}
		}
		fmt.Printf("Scaled down workers: %d -> %d (queue depth: %d)\n", currentWorkers, targetWorkers, queueLen)
	}
}

// workerPoolManager monitors the system and adjusts workers accordingly
func (ps *ParallelFileScanner) workerPoolManager() {
	defer ps.wg.Done()
	
	// Check worker pool every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	// Initial metrics
	var lastFilesProcessed int64
	var lastCheck time.Time = time.Now()
	
	for {
		select {
		case <-ticker.C:
			// Get current metrics
			currentFiles := ps.filesProcessed.Load()
			currentTime := time.Now()
			
			// Calculate throughput (files per second)
			timeDelta := currentTime.Sub(lastCheck).Seconds()
			filesDelta := currentFiles - lastFilesProcessed
			throughput := float64(filesDelta) / timeDelta
			
			// Log performance metrics
			queueLen := len(ps.workQueue)
			activeWorkers := ps.activeWorkers.Load()
			
			fmt.Printf("Worker Pool Status: Active: %d, Queue: %d, Throughput: %.2f files/sec\n", 
				activeWorkers, queueLen, throughput)
			
			// Adjust workers based on current conditions
			ps.adjustWorkers()
			
			// Update metrics for next iteration
			lastFilesProcessed = currentFiles
			lastCheck = currentTime
			
		case <-ps.ctx.Done():
			return
		}
	}
}

// GetWorkerStats returns current worker pool statistics
func (ps *ParallelFileScanner) GetWorkerStats() (active, min, max, queueLen int) {
	return int(ps.activeWorkers.Load()), ps.minWorkers, ps.maxWorkers, len(ps.workQueue)
}

// BatchProcessor methods

func (bp *BatchProcessor) AddMediaFile(file *database.MediaFile) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.mediaFiles = append(bp.mediaFiles, file)
	
	if len(bp.mediaFiles) >= bp.batchSize {
		go bp.Flush()
	}
}

func (bp *BatchProcessor) AddMusicMetadata(meta *database.MusicMetadata) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.musicMetas = append(bp.musicMetas, meta)
}

func (bp *BatchProcessor) AddMusicMetadataWithPath(meta *database.MusicMetadata, path string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.musicMetas = append(bp.musicMetas, meta)
	bp.metaToPath[meta] = path
}

func (bp *BatchProcessor) FlushIfNeeded() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	if len(bp.mediaFiles) > 0 || len(bp.musicMetas) > 0 {
		return bp.flushInternal()
	}
	return nil
}

func (bp *BatchProcessor) Flush() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	return bp.flushInternal()
}

func (bp *BatchProcessor) flushInternal() error {
	if len(bp.mediaFiles) == 0 && len(bp.musicMetas) == 0 {
		return nil
	}
	
	// Use transaction for batch operations
	err := bp.db.Transaction(func(tx *gorm.DB) error {
		// Create a map to track path -> media file for metadata linking
		pathToMediaFile := make(map[string]*database.MediaFile)
		
		// Batch upsert media files first
		if len(bp.mediaFiles) > 0 {
			// Build path map before insertion
			for _, mediaFile := range bp.mediaFiles {
				pathToMediaFile[mediaFile.Path] = mediaFile
			}
			
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "path"}},
				DoUpdates: clause.AssignmentColumns([]string{"size", "hash", "last_seen"}),
			}).CreateInBatches(bp.mediaFiles, 100).Error; err != nil {
				return err
			}
			
			// After insertion, get the actual IDs from database for all paths
			var savedFiles []database.MediaFile
			paths := make([]string, 0, len(bp.mediaFiles))
			for _, mf := range bp.mediaFiles {
				paths = append(paths, mf.Path)
			}
			
			if err := tx.Where("path IN ?", paths).Find(&savedFiles).Error; err != nil {
				return fmt.Errorf("failed to retrieve saved media files: %w", err)
			}
			
			// Update the path map with actual database IDs
			for _, savedFile := range savedFiles {
				if originalFile, exists := pathToMediaFile[savedFile.Path]; exists {
					originalFile.ID = savedFile.ID
				}
			}
		}
		
		// Now process music metadata with correct media file IDs
		if len(bp.musicMetas) > 0 {
			// Update metadata with correct media file IDs using path mapping
			validMetas := make([]*database.MusicMetadata, 0, len(bp.musicMetas))
			
			for _, musicMeta := range bp.musicMetas {
				// Find the corresponding media file by path
				if path, exists := bp.metaToPath[musicMeta]; exists {
					if mediaFile, pathExists := pathToMediaFile[path]; pathExists && mediaFile.ID > 0 {
						// Update the metadata with the correct media file ID
						musicMeta.MediaFileID = mediaFile.ID
						validMetas = append(validMetas, musicMeta)
						fmt.Printf("DEBUG: Linked metadata for %s to media file ID %d\n", path, mediaFile.ID)
					} else {
						fmt.Printf("WARNING: Could not find media file for path %s\n", path)
					}
				} else if musicMeta.MediaFileID > 0 {
					// Metadata already has a valid ID (from cache or previous processing)
					validMetas = append(validMetas, musicMeta)
				} else {
					fmt.Printf("WARNING: Music metadata has no path mapping and MediaFileID = 0, skipping\n")
				}
			}
			
			if len(validMetas) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "media_file_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"title", "artist", "album", "year", "genre", "duration", "bitrate"}),
				}).CreateInBatches(validMetas, 100).Error; err != nil {
					return err
				}
				fmt.Printf("DEBUG: Successfully inserted %d music metadata records\n", len(validMetas))
				
				// Save artwork for metadata that has artwork data
				for _, meta := range validMetas {
					if meta.HasArtwork && len(meta.ArtworkData) > 0 && meta.MediaFileID > 0 {
						err := metadata.SaveArtworkWithID(meta.MediaFileID, meta.ArtworkData, meta.ArtworkExt)
						if err != nil {
							fmt.Printf("Warning: failed to save artwork for media file ID %d: %v\n", meta.MediaFileID, err)
						} else {
							fmt.Printf("Successfully saved artwork for media file ID %d\n", meta.MediaFileID)
						}
					}
				}
			}
		}
		
		return nil
	})
	
	if err == nil {
		// Clear batches
		bp.mediaFiles = bp.mediaFiles[:0]
		bp.musicMetas = bp.musicMetas[:0]
		// Clear path mapping
		for k := range bp.metaToPath {
			delete(bp.metaToPath, k)
		}
	}
	
	return err
}

// directoryWorker processes directories from the directory queue
func (ps *ParallelFileScanner) directoryWorker(workerID int) {
	defer ps.wg.Done()

	fmt.Printf("DEBUG: directoryWorker %d started\n", workerID)

	for {
		select {
		case <-ps.ctx.Done():
			fmt.Printf("DEBUG: directoryWorker %d - context cancelled\n", workerID)
			return
		case dirWork, ok := <-ps.dirQueue:
			if !ok {
				fmt.Printf("DEBUG: directoryWorker %d - directory queue closed\n", workerID)
				return
			}
			
			fmt.Printf("DEBUG: directoryWorker %d - processing directory: %s\n", workerID, dirWork.path)
			ps.activeDirWorkers.Add(1)
			
			ps.processDirWork(dirWork)
			
			ps.activeDirWorkers.Add(-1)
			fmt.Printf("DEBUG: directoryWorker %d - finished processing directory: %s\n", workerID, dirWork.path)
		}
	}
}

// processDirWork processes a single directory
func (ps *ParallelFileScanner) processDirWork(work dirWork) {
	fmt.Printf("DEBUG: Processing directory: %s\n", work.path)
	entries, err := os.ReadDir(work.path)
	if err != nil {
		fmt.Printf("ERROR: Failed to read directory %s: %v\n", work.path, err)
		ps.errorsCount.Add(1)
		return
	}
	
	fmt.Printf("DEBUG: Found %d entries in directory %s\n", len(entries), work.path)
	
	for _, entry := range entries {
		// Check for cancellation
		select {
		case <-ps.ctx.Done():
			return
		default:
		}
		
		fullPath := filepath.Join(work.path, entry.Name())
		
		if entry.IsDir() {
			fmt.Printf("DEBUG: Found subdirectory: %s\n", fullPath)
			fmt.Printf("DEBUG: Adding subdirectory to queue: %s\n", fullPath)
			// Add subdirectory to directory queue for parallel processing
			select {
			case ps.dirQueue <- dirWork{
				path:      fullPath,
				libraryID: work.libraryID,
			}:
				fmt.Printf("DEBUG: Successfully added subdirectory to queue: %s\n", fullPath)
			case <-ps.ctx.Done():
				return
			case <-time.After(5 * time.Second):
				fmt.Printf("WARNING: Timeout adding subdirectory to queue (queue may be full): %s\n", fullPath)
				return
			}
		} else {
			fmt.Printf("DEBUG: Found file: %s\n", fullPath)
			// Quick file type check before getting full file info
			// Use metadata.IsMediaLibraryFile for consistency with countFilesToScan
			if !metadata.IsMediaLibraryFile(fullPath) {
				fmt.Printf("DEBUG: Skipping non-media file: %s\n", fullPath)
				continue
			}
			
			fmt.Printf("DEBUG: Found media file: %s\n", fullPath)
			
			// Get file info
			info, err := entry.Info()
			if err != nil {
				fmt.Printf("ERROR: Failed to get file info for %s: %v\n", fullPath, err)
				ps.errorsCount.Add(1)
				continue
			}
			
			// Submit file work to file processing queue
			fmt.Printf("DEBUG: Adding file to work queue: %s (size: %d)\n", fullPath, info.Size())
			select {
			case ps.workQueue <- scanWork{
				path:      fullPath,
				info:      info,
				libraryID: work.libraryID,
			}:
				fmt.Printf("DEBUG: Successfully added file to work queue: %s\n", fullPath)
			case <-ps.ctx.Done():
				return
			}
		}
	}
	fmt.Printf("DEBUG: Finished processing directory: %s\n", work.path)
}

// dirQueueManager monitors directory workers and closes directory queue when done
func (ps *ParallelFileScanner) dirQueueManager() {
	defer ps.wg.Done()
	defer close(ps.dirQueue)
	
	fmt.Printf("DEBUG: dirQueueManager started\n")
	
	// Add a startup grace period to allow scanDirectory to add initial work
	startupGracePeriod := 5 * time.Second
	startTime := time.Now()
	
	// Wait for all directory workers to finish
	// Use a more robust approach to avoid premature closure
	idleCount := 0
	const maxIdleChecks = 100 // Reduced from 600 to 10 seconds (100 * 100ms)
	
	for {
		select {
		case <-ps.ctx.Done():
			fmt.Printf("DEBUG: dirQueueManager - context cancelled, exiting\n")
			return
		case <-time.After(100 * time.Millisecond):
			// Check if directory queue is empty and all directory workers are idle
			queueLen := len(ps.dirQueue)
			activeWorkers := ps.activeDirWorkers.Load()
			
			// During startup grace period, don't start counting idle time
			if time.Since(startTime) < startupGracePeriod {
				fmt.Printf("DEBUG: dirQueueManager - in startup grace period (%.1fs remaining)\n", 
					startupGracePeriod.Seconds()-time.Since(startTime).Seconds())
				continue
			}
			
			// fmt.Printf("DEBUG: dirQueueManager - checking status: queue=%d, workers=%d, idleCount=%d\n",
			// 	queueLen, activeWorkers, idleCount)

			if queueLen == 0 && activeWorkers == 0 {
				idleCount++
				fmt.Printf("DEBUG: dirQueueManager - idle check %d/%d (queue: %d, workers: %d)\n", 
					idleCount, maxIdleChecks, queueLen, activeWorkers)
				
				if idleCount >= maxIdleChecks {
					// All directory scanning is done, close the directory queue
					fmt.Printf("DEBUG: dirQueueManager - closing directory queue after %d idle checks\n", idleCount)
					return
				}
			} else {
				// Reset idle count if there's activity
				if idleCount > 0 {
					fmt.Printf("DEBUG: dirQueueManager - resetting idle count (queue: %d, workers: %d)\n", 
						queueLen, activeWorkers)
				}
				idleCount = 0
			}
		}
	}
}

// workQueueCloser waits for directory scanning to complete and then closes the work queue
func (ps *ParallelFileScanner) workQueueCloser() {
	defer ps.wg.Done()
	defer close(ps.workQueue)
	
	// Wait for directory scanning to complete
	for {
		select {
		case <-ps.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Check if all directory workers are idle and directory queue is empty
			if ps.activeDirWorkers.Load() == 0 && len(ps.dirQueue) == 0 {
				// Give a small grace period for any final file submissions
				time.Sleep(200 * time.Millisecond)
				// Double-check the condition
				if ps.activeDirWorkers.Load() == 0 && len(ps.dirQueue) == 0 {
					return
				}
			}
		}
	}
}
