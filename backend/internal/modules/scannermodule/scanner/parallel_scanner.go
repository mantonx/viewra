package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/metadata"
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
	mediaFile    *database.MediaFile
	musicMeta    *database.MusicMetadata
	path         string
	error        error
}

// BatchProcessor handles batch database operations
type BatchProcessor struct {
	db           *gorm.DB
	batchSize    int
	timeout      time.Duration
	mediaFiles   []*database.MediaFile
	musicMetas   []*database.MusicMetadata
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
func NewParallelFileScanner(db *gorm.DB, jobID uint, eventBus events.EventBus) *ParallelFileScanner {
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
		workerCount:   workerCount,
		minWorkers:    minWorkers,
		maxWorkers:    maxWorkers,
		dirWorkerCount: dirWorkerCount,
		workerExitChan: make(chan int, maxWorkers),
		workQueue:     make(chan scanWork, workerCount*100), // Buffered queue
		dirQueue:      make(chan dirWork, dirWorkerCount*50), // Buffered directory queue
		resultQueue:   make(chan *scanResult, workerCount*10),
		errorQueue:    make(chan error, workerCount),
		batchSize:     100, // Process files in batches of 100
		batchTimeout:  5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		fileCache:     &FileCache{cache: make(map[string]*database.MediaFile)},
		metadataCache: &MetadataCache{
			cache: make(map[string]*database.MusicMetadata),
			ttl:   30 * time.Minute,
		},
	}
	
	// Initialize batch processor
	scanner.batchProcessor = &BatchProcessor{
		db:         db,
		batchSize:  scanner.batchSize,
		timeout:    scanner.batchTimeout,
		mediaFiles: make([]*database.MediaFile, 0, scanner.batchSize),
		musicMetas: make([]*database.MusicMetadata, 0, scanner.batchSize),
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
	
	// Update job status to running
	if err := utils.UpdateJobStatus(ps.db, ps.jobID, utils.StatusRunning, ""); err != nil {
		return fmt.Errorf("failed to update job status to running: %w", err)
	}
	
	// Pre-load existing files into cache for fast lookup
	if err := ps.preloadFileCache(libraryID); err != nil {
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
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
	
	// Start directory scanner
	ps.wg.Add(1)
	go ps.scanDirectory(libraryID)
	
	// Wait for completion
	ps.wg.Wait()
	
	// Final batch flush
	if err := ps.batchProcessor.Flush(); err != nil {
		fmt.Printf("Error flushing final batch: %v\n", err)
	}
	
	// Update final job status
	ps.updateFinalStatus()
	
	return nil
}

// Resume resumes a previously paused scan
func (ps *ParallelFileScanner) Resume(libraryID uint) error {
	// This method is called when resuming, but Start handles both new and resumed scans
	return ps.Start(libraryID)
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
	filesProcessed := ps.filesProcessed.Load()
	bytesProcessed := ps.bytesProcessed.Load()
	errorsCount := ps.errorsCount.Load()
	
	// Update scan job with final stats
	now := time.Now()
	ps.scanJob.FilesProcessed = int(filesProcessed)
	ps.scanJob.BytesProcessed = bytesProcessed
	ps.scanJob.Progress = 100
	ps.scanJob.CompletedAt = &now
	ps.scanJob.UpdatedAt = now
	
	// Set status based on whether there were errors
	if errorsCount > 0 {
		ps.scanJob.Status = "completed_with_errors"
		ps.scanJob.ErrorMessage = fmt.Sprintf("Completed with %d errors", errorsCount)
	} else {
		ps.scanJob.Status = "completed"
		ps.scanJob.ErrorMessage = ""
	}
	
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		fmt.Printf("Failed to update final scan job status: %v\n", err)
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

// scanDirectory initiates parallel directory walking
func (ps *ParallelFileScanner) scanDirectory(libraryID uint) {
	defer ps.wg.Done()
	
	libraryPath := ps.scanJob.Library.Path
	basePath, err := ps.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		ps.errorQueue <- fmt.Errorf("failed to resolve directory: %w", err)
		return
	}
	
	// Start with the root directory in the queue
	select {
	case ps.dirQueue <- dirWork{
		path:      basePath,
		libraryID: libraryID,
	}:
	case <-ps.ctx.Done():
		return
	}
	
	// This method just initiates the scanning; the dirQueueManager will handle closing
}

// worker processes files from the work queue
func (ps *ParallelFileScanner) worker(id int) {
	defer ps.wg.Done()
	defer ps.activeWorkers.Add(-1) // Decrement active worker count when exiting
	
	ps.activeWorkers.Add(1) // Increment active worker count
	
	for {
		select {
		case work, ok := <-ps.workQueue:
			if !ok {
				// Work queue closed, exit
				return
			}
			
			// Check for cancellation
			select {
			case <-ps.ctx.Done():
				return
			default:
			}
			
			result := ps.processFile(work)
			
			select {
			case ps.resultQueue <- result:
			case <-ps.ctx.Done():
				return
			}
			
		case exitID := <-ps.workerExitChan:
			if exitID == id {
				// This worker was signaled to exit
				return
			}
			// Put the signal back if it's not for this worker
			ps.workerExitChan <- exitID
			
		case <-ps.ctx.Done():
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
		// File hasn't changed, skip processing
		ps.filesProcessed.Add(1)
		ps.bytesProcessed.Add(work.info.Size())
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
	
	// Extract metadata for music files
	if metadata.IsMusicFile(work.path) {
		// Check metadata cache
		ps.metadataCache.mu.RLock()
		cachedMeta, metaExists := ps.metadataCache.cache[hash]
		ps.metadataCache.mu.RUnlock()
		
		if !metaExists {
			musicMeta, err := metadata.ExtractMusicMetadata(work.path, mediaFile)
			if err != nil {
				fmt.Printf("Failed to extract metadata from %s: %v\n", work.path, err)
			} else {
				result.musicMeta = musicMeta
				
				// Cache the metadata
				ps.metadataCache.mu.Lock()
				ps.metadataCache.cache[hash] = musicMeta
				ps.metadataCache.mu.Unlock()
			}
		} else {
			result.musicMeta = cachedMeta
		}
	}
	
	// Update metrics
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

// resultProcessor handles results from workers
func (ps *ParallelFileScanner) resultProcessor() {
	defer ps.wg.Done()
	defer close(ps.resultQueue)
	
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
				ps.batchProcessor.AddMusicMetadata(result.musicMeta)
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
	
	// Update progress estimator
	ps.progressEstimator.Update(filesProcessed, bytesProcessed)
	
	// Calculate progress percentage (simplified for now)
	progress := 0
	if ps.scanJob.FilesFound > 0 {
		progress = int((float64(filesProcessed) / float64(ps.scanJob.FilesFound)) * 100)
	}
	
	// Update scan job
	ps.scanJob.FilesProcessed = int(filesProcessed)
	ps.scanJob.Progress = progress
	ps.scanJob.UpdatedAt = time.Now()
	
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		fmt.Printf("Failed to update scan job progress: %v\n", err)
	}
	
	// Publish progress event
	if ps.eventBus != nil && filesProcessed%100 == 0 {
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
		// Batch upsert media files
		if len(bp.mediaFiles) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "path"}},
				DoUpdates: clause.AssignmentColumns([]string{"size", "hash", "last_seen"}),
			}).CreateInBatches(bp.mediaFiles, 100).Error; err != nil {
				return err
			}
		}
		
		// Batch insert music metadata
		if len(bp.musicMetas) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "media_file_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"title", "artist", "album", "year", "genre", "duration", "bitrate"}),
			}).CreateInBatches(bp.musicMetas, 100).Error; err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err == nil {
		// Clear batches
		bp.mediaFiles = bp.mediaFiles[:0]
		bp.musicMetas = bp.musicMetas[:0]
	}
	
	return err
}

// directoryWorker processes directories from the directory queue
func (ps *ParallelFileScanner) directoryWorker(id int) {
	defer ps.wg.Done()
	defer ps.activeDirWorkers.Add(-1)
	
	ps.activeDirWorkers.Add(1)
	
	for {
		select {
		case dirWork, ok := <-ps.dirQueue:
			if !ok {
				// Directory queue closed, exit
				return
			}
			
			// Check for cancellation
			select {
			case <-ps.ctx.Done():
				return
			default:
			}
			
			ps.processDirWork(dirWork)
			
		case <-ps.ctx.Done():
			return
		}
	}
}

// processDirWork processes a single directory
func (ps *ParallelFileScanner) processDirWork(work dirWork) {
	entries, err := os.ReadDir(work.path)
	if err != nil {
		ps.errorsCount.Add(1)
		return
	}
	
	for _, entry := range entries {
		// Check for cancellation
		select {
		case <-ps.ctx.Done():
			return
		default:
		}
		
		fullPath := filepath.Join(work.path, entry.Name())
		
		if entry.IsDir() {
			// Add subdirectory to directory queue for parallel processing
			select {
			case ps.dirQueue <- dirWork{
				path:      fullPath,
				libraryID: work.libraryID,
			}:
			case <-ps.ctx.Done():
				return
			}
		} else {
			// Quick file type check before getting full file info
			if !utils.IsMediaFile(fullPath) {
				continue
			}
			
			// Get file info
			info, err := entry.Info()
			if err != nil {
				ps.errorsCount.Add(1)
				continue
			}
			
			// Submit file work to file processing queue
			select {
			case ps.workQueue <- scanWork{
				path:      fullPath,
				info:      info,
				libraryID: work.libraryID,
			}:
			case <-ps.ctx.Done():
				return
			}
		}
	}
}

// dirQueueManager monitors directory workers and closes directory queue when done
func (ps *ParallelFileScanner) dirQueueManager() {
	defer ps.wg.Done()
	defer close(ps.dirQueue)
	
	// Wait for all directory workers to finish
	for {
		select {
		case <-ps.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Check if directory queue is empty and all directory workers are idle
			if len(ps.dirQueue) == 0 && ps.activeDirWorkers.Load() == 0 {
				// All directory scanning is done, close the directory queue
				return
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
