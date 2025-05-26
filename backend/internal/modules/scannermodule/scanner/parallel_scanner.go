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
	
	// Worker management
	workerCount      int
	workQueue        chan scanWork
	resultQueue      chan *scanResult
	errorQueue       chan error
	
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
	flushTimer   *time.Timer
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
	
	scanner := &ParallelFileScanner{
		db:            db,
		jobID:         jobID,
		eventBus:      eventBus,
		pathResolver:  utils.NewPathResolver(),
		workerCount:   workerCount,
		workQueue:     make(chan scanWork, workerCount*100), // Buffered queue
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
	
	return scanner
}

// Start begins the parallel scanning process
func (ps *ParallelFileScanner) Start(libraryID uint) error {
	ps.startTime = time.Now()
	
	// Load scan job
	if err := ps.loadScanJob(); err != nil {
		return err
	}
	
	// Pre-load existing files into cache for fast lookup
	if err := ps.preloadFileCache(libraryID); err != nil {
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
	}
	
	// Start workers
	for i := 0; i < ps.workerCount; i++ {
		ps.wg.Add(1)
		go ps.worker(i)
	}
	
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
		
		if len(chunk) < chunkSize {
			break
		}
	}
	
	fmt.Printf("Preloaded %d existing files into cache\n", len(existingFiles))
	return nil
}

// scanDirectory walks the directory tree and submits work to the queue
func (ps *ParallelFileScanner) scanDirectory(libraryID uint) {
	defer ps.wg.Done()
	defer close(ps.workQueue)
	
	libraryPath := ps.scanJob.Library.Path
	basePath, err := ps.pathResolver.ResolveDirectory(libraryPath)
	if err != nil {
		ps.errorQueue <- fmt.Errorf("failed to resolve directory: %w", err)
		return
	}
	
	// Use filepath.WalkDir for better performance than filepath.Walk
	err = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		// Check for cancellation
		select {
		case <-ps.ctx.Done():
			return fmt.Errorf("scan cancelled")
		default:
		}
		
		if err != nil {
			ps.errorsCount.Add(1)
			return nil // Continue scanning despite errors
		}
		
		if d.IsDir() {
			return nil
		}
		
		// Quick file type check before getting full file info
		if !utils.IsMediaFile(path) {
			return nil
		}
		
		// Get file info
		info, err := d.Info()
		if err != nil {
			ps.errorsCount.Add(1)
			return nil
		}
		
		// Submit work to queue
		select {
		case ps.workQueue <- scanWork{
			path:      path,
			info:      info,
			libraryID: libraryID,
		}:
		case <-ps.ctx.Done():
			return fmt.Errorf("scan cancelled")
		}
		
		return nil
	})
	
	if err != nil && err.Error() != "scan cancelled" {
		ps.errorQueue <- err
	}
}

// worker processes files from the work queue
func (ps *ParallelFileScanner) worker(id int) {
	defer ps.wg.Done()
	
	for work := range ps.workQueue {
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
			fmt.Sprintf("Processed %d files (%.2f GB)", filesProcessed, float64(bytesProcessed)/(1024*1024*1024)),
		)
		event.Data = map[string]interface{}{
			"jobId":          ps.jobID,
			"filesProcessed": filesProcessed,
			"bytesProcessed": bytesProcessed,
			"errorsCount":    errorsCount,
			"progress":       progress,
		}
		ps.eventBus.PublishAsync(event)
	}
}

// Pause pauses the scanner
func (ps *ParallelFileScanner) Pause() {
	ps.cancel()
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
