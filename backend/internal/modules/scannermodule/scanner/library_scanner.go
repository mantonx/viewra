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
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/music"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// LibraryScanner implements a high-performance parallel file scanner
type LibraryScanner struct {
	db                *gorm.DB
	jobID             uint
	eventBus          events.EventBus
	scanJob           *database.ScanJob
	pathResolver      *utils.PathResolver
	progressEstimator *ProgressEstimator
	systemMonitor     *SystemLoadMonitor
	pluginManager     plugins.Manager

	// Plugin integration
	pluginRouter      *PluginRouter
	corePluginsManager *plugins.CorePluginsManager
	mediaManager      *plugins.MediaManager

	// Worker management
	workerCount    int
	minWorkers     int          // Minimum number of workers
	maxWorkers     int          // Maximum number of workers
	activeWorkers  atomic.Int32 // Current number of active workers
	workerExitChan chan int     // Channel for signaling workers to exit
	workQueue      chan scanWork
	resultQueue    chan *scanResult
	errorQueue     chan error

	// Directory walking - parallel implementation
	dirWorkerCount   int          // Number of directory walking workers
	dirQueue         chan dirWork // Queue for directory scanning work
	activeDirWorkers atomic.Int32 // Current number of active directory workers

	// Batch processing
	batchSize      int
	batchTimeout   time.Duration
	batchProcessor *BatchProcessor

	// State management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Performance metrics
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errorsCount    atomic.Int64
	filesSkipped   atomic.Int64 // Files that couldn't be processed
	startTime      time.Time

	// Cache for improved performance
	fileCache     *FileCache
	metadataCache *MetadataCache

	// Control flags
	wasExplicitlyPaused atomic.Bool
	dbMutex             sync.Mutex // Mutex for serializing DB write operations
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
	metadata         interface{} // Generic metadata that can be MusicMetadata, VideoMetadata, etc.
	metadataType     string      // Type identifier: "music", "video", "image", etc.
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
	metadataItems []MetadataItem // Generic metadata items
	mu           sync.Mutex
}

// MetadataItem represents a generic metadata item with its type
type MetadataItem struct {
	Data interface{} // The actual metadata (MusicMetadata, VideoMetadata, etc.)
	Type string      // Type identifier: "music", "video", "image", etc.
	Path string      // File path for linking
}

// FileCache provides fast lookups for existing files
type FileCache struct {
	cache map[string]*database.MediaFile
	mu    sync.RWMutex
}

// MetadataCache caches metadata extraction results (now generic)
type MetadataCache struct {
	cache map[string]interface{} // Generic metadata cache
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewLibraryScanner creates a new parallel file scanner with optimizations
func NewLibraryScanner(db *gorm.DB, jobID uint, eventBus events.EventBus, pluginManager plugins.Manager) *LibraryScanner {
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

	scanner := &LibraryScanner{
		db:             db,
		jobID:          jobID,
		eventBus:       eventBus,
		pathResolver:   utils.NewPathResolver(),
		pluginManager:  pluginManager,
		workerCount:    workerCount,
		minWorkers:     minWorkers,
		maxWorkers:     maxWorkers,
		dirWorkerCount: dirWorkerCount,
		workerExitChan: make(chan int, maxWorkers),
		workQueue:      make(chan scanWork, workerCount*100),   // Buffered queue
		dirQueue:       make(chan dirWork, dirWorkerCount*5000), // Buffered queue with larger buffer
		resultQueue:    make(chan *scanResult, workerCount*10),
		errorQueue:     make(chan error, workerCount),
		batchSize:      100, // Process files in batches of 100
		batchTimeout:   5 * time.Second,
		ctx:            ctx,
		cancel:         cancel,
		fileCache: &FileCache{
			cache: make(map[string]*database.MediaFile),
			mu:    sync.RWMutex{},
		},
	}

	// Initialize plugin integration components
	scanner.pluginRouter = NewPluginRouter(pluginManager)
	
	// Initialize core media plugins
	coreManager := plugins.NewCorePluginsManager()
	
	// Register music plugin factory
	coreManager.RegisterPluginFactory("music", func() plugins.CoreMediaPlugin {
		return music.NewMusicPlugin()
	})
	
	// Initialize core plugins
	if err := coreManager.InitializeCorePlugins(); err != nil {
		fmt.Printf("WARNING: Failed to initialize core plugins: %v\n", err)
	}
	
	scanner.corePluginsManager = coreManager

	// Initialize batch processor
	scanner.batchProcessor = &BatchProcessor{
		db:           db,
		batchSize:    scanner.batchSize,
		timeout:      scanner.batchTimeout,
		mediaFiles:   make([]*database.MediaFile, 0, scanner.batchSize),
		metadataItems: make([]MetadataItem, 0, scanner.batchSize),
	}

	// Initialize MediaManager
	scanner.mediaManager = plugins.NewMediaManager(db)

	// Initialize metadata cache
	scanner.metadataCache = &MetadataCache{
		cache: make(map[string]interface{}),
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
func (ps *LibraryScanner) Start(libraryID uint) error {
	ps.startTime = time.Now()
	logger.Info("Starting library scan", "library_id", libraryID)

	ps.dbMutex.Lock()
	if err := ps.loadScanJob(); err != nil {
		ps.dbMutex.Unlock()
		return fmt.Errorf("failed to load scan job: %w", err)
	}

	var library database.MediaLibrary
	if err := ps.db.First(&library, libraryID).Error; err != nil {
		ps.dbMutex.Unlock()
		return fmt.Errorf("failed to load library: %w", err)
	}
	ps.dbMutex.Unlock()

	if library.Path == "" {
		return fmt.Errorf("library path is empty")
	}

	ps.dbMutex.Lock()
	ps.scanJob.Status = "running"
	ps.scanJob.StartedAt = &ps.startTime
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		ps.dbMutex.Unlock()
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	ps.dbMutex.Unlock()

	event := events.Event{
		Type:    "scan.started",
		Source:  "scanner",
		Title:   "Scan Started",
		Message: fmt.Sprintf("Started scanning library %d", libraryID),
		Data: map[string]interface{}{
			"job_id":     ps.jobID,
			"library_id": libraryID,
			"path":       library.Path,
		},
	}
	ps.eventBus.PublishAsync(event)

	if ps.pluginRouter != nil {
		ps.pluginRouter.CallOnScanStarted(ps.jobID, libraryID, library.Path)
	}

	if err := ps.preloadFileCache(libraryID); err != nil {
		logger.Warn("Failed to preload file cache", "error", err)
	}

	fileCount, err := ps.countFilesToScan(library.Path)
	if err != nil {
		logger.Warn("Failed to count files for progress estimation", "error", err)
		fileCount = 1000
	}
	ps.progressEstimator.SetTotal(int64(fileCount), 0)

	// Start all workers and processors
	ps.wg.Add(1)
	go ps.workerPoolManager()

	ps.wg.Add(1)
	go ps.resultProcessor()

	ps.wg.Add(1)
	go ps.batchFlusher()

	ps.wg.Add(1)
	go ps.dirQueueManager()

	ps.wg.Add(1)
	go ps.workQueueCloser()

	ps.wg.Add(1)
	go ps.updateProgress()

	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
	}

	ps.wg.Add(1)
	go ps.scanDirectory(libraryID)

	ps.wg.Wait()

	ps.updateFinalStatus()

	return nil
}

// Resume resumes a paused scan job
func (ps *LibraryScanner) Resume(libraryID uint) error {
	// Load scan job
	if err := ps.loadScanJob(); err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}

	// Check if job is in correct state for resume
	if ps.scanJob.Status != "paused" {
		return fmt.Errorf("scan job is not in paused state, current status: %s", ps.scanJob.Status)
	}

	// Load library
	var library database.MediaLibrary
	if err := ps.db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("failed to load library: %w", err)
	}

	if library.Path == "" {
		return fmt.Errorf("library path is empty")
	}

	// Update job status to running
	ps.scanJob.Status = "running"
	resumeTime := time.Now()
	ps.scanJob.StartedAt = &resumeTime // Reset started time for resume
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}

	// Emit scan resumed event
	event := events.Event{
		Type:    "scan.resumed",
		Source:  "scanner",
		Title:   "Scan Resumed",
		Message: fmt.Sprintf("Resumed scanning library %d", libraryID),
		Data: map[string]interface{}{
			"job_id":     ps.jobID,
			"library_id": libraryID,
			"path":       library.Path,
		},
	}
	ps.eventBus.PublishAsync(event)

	// Reset internal state for resume
	ps.startTime = resumeTime
	ps.filesProcessed.Store(0)
	ps.bytesProcessed.Store(0)
	ps.errorsCount.Store(0)
	ps.filesSkipped.Store(0)

	// Preload file cache for performance
	if err := ps.preloadFileCache(libraryID); err != nil {
		// Log warning but continue
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
	}

	// Count files for progress estimation
	fileCount, err := ps.countFilesToScan(library.Path)
	if err != nil {
		fmt.Printf("Warning: Failed to count files for progress estimation: %v\n", err)
		fileCount = 1000 // Fallback estimate
	}
	ps.progressEstimator.SetTotal(int64(fileCount), 0)

	// Start the worker pool
	ps.wg.Add(1)
	go ps.workerPoolManager()

	// Start result processor
	ps.wg.Add(1)
	go ps.resultProcessor()

	// Start batch flusher
	ps.wg.Add(1)
	go ps.batchFlusher()

	// Start queue managers
	ps.wg.Add(1)
	go ps.dirQueueManager()

	ps.wg.Add(1)
	go ps.workQueueCloser()

	// Start progress updates
	ps.wg.Add(1)
	go ps.updateProgress()

	// Start directory workers
	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
	}

	// Kick off the scanning process
	ps.wg.Add(1)
	go ps.scanDirectory(libraryID)

	// Wait for completion or cancellation
	ps.wg.Wait()

	// Update final status
	ps.updateFinalStatus()

	return nil
}

func (ps *LibraryScanner) loadScanJob() error {
	return ps.db.First(&ps.scanJob, ps.jobID).Error
}

func (ps *LibraryScanner) updateFinalStatus() {
	ps.dbMutex.Lock()
	defer ps.dbMutex.Unlock()

	// Load latest scan job state from the database
	var currentJobState database.ScanJob
	if err := ps.db.First(&currentJobState, ps.jobID).Error; err != nil {
		fmt.Printf("Warning: Failed to load scan job %d for final status update: %v\n", ps.jobID, err)
		if ps.scanJob != nil {
			ps.scanJob.Status = "failed"
			ps.scanJob.ErrorMessage = "failed to load job for final status update"
		}
		return
	}

	// If the job was explicitly paused (flag set by Pause()), it should remain paused.
	if ps.wasExplicitlyPaused.Load() {
		fmt.Printf("Scan job %d was explicitly paused, final status will be 'paused'.\n", ps.jobID)
		if currentJobState.Status != "paused" {
			// If DB isn't already paused, update it. Pause() should have done this, but as a safeguard.
			if err := utils.UpdateJobStatus(ps.db, ps.jobID, "paused", ""); err != nil {
				fmt.Printf("Warning: Failed to ensure job %d is paused in DB during final status update: %v\n", ps.jobID, err)
			}
		}
		if ps.scanJob != nil {
			ps.scanJob.Status = "paused"
		}
		return
	}

	// If the job was explicitly paused by DB status (e.g. recovered orphaned job set to paused)
	// and not by the wasExplicitlyPaused flag (which means it wasn't an active pause during this run)
	if currentJobState.Status == "paused" {
		fmt.Printf("Scan job %d found as paused in DB, final status remains paused.\n", ps.jobID)
		if ps.scanJob != nil {
			ps.scanJob.Status = "paused"
		}
		return
	}

	// Only proceed to mark as completed if it was genuinely running and not interrupted by pause
	if currentJobState.Status == "running" { // Check DB status
		endTime := time.Now()
		duration := endTime.Sub(ps.startTime)

		// Update the job in the database
		updateData := map[string]interface{}{
			"status":          "completed",
			"completed_at":    &endTime,
			"files_processed": int(ps.filesProcessed.Load()),
			"bytes_processed": ps.bytesProcessed.Load(),
			"error_message":   "",                       // Clear any previous error
		}

		if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(updateData).Error; err != nil {
			fmt.Printf("Warning: Failed to update final scan job status to completed for job %d: %v\n", ps.jobID, err)
			// Update our in-memory copy to reflect failure to update DB
			if ps.scanJob != nil {
				ps.scanJob.Status = "failed"
				ps.scanJob.ErrorMessage = "failed to save completed status"
			}
			return
		}

		// Update in-memory ps.scanJob to match
		if ps.scanJob != nil {
			ps.scanJob.Status = "completed"
			ps.scanJob.CompletedAt = &endTime
			ps.scanJob.FilesProcessed = int(ps.filesProcessed.Load())
			ps.scanJob.BytesProcessed = ps.bytesProcessed.Load()
			ps.scanJob.ErrorMessage = ""
		}

		// Emit scan completed event
		event := events.Event{
			Type:    "scan.completed",
			Source:  "scanner",
			Title:   "Scan Completed",
			Message: fmt.Sprintf("Completed scanning job %d", ps.jobID),
			Data: map[string]interface{}{
				"job_id":          ps.jobID,
				"files_processed": ps.filesProcessed.Load(),
				"files_skipped":   ps.filesSkipped.Load(),
				"errors_count":    ps.errorsCount.Load(),
				"duration_ms":     duration.Milliseconds(),
				"throughput_fps":  float64(ps.filesProcessed.Load()) / duration.Seconds(),
			},
		}
		ps.eventBus.PublishAsync(event)

		// Call plugin hooks for scan completed  
		if ps.pluginRouter != nil {
			// Get the library ID from the scan job
			libraryID := uint(0)
			if ps.scanJob != nil {
				libraryID = ps.scanJob.LibraryID
			}
			
			// Prepare stats for plugin hooks
			stats := map[string]interface{}{
				"files_processed": ps.filesProcessed.Load(),
				"files_skipped":   ps.filesSkipped.Load(),
				"errors_count":    ps.errorsCount.Load(),
				"duration_ms":     duration.Milliseconds(),
				"throughput_fps":  float64(ps.filesProcessed.Load()) / duration.Seconds(),
			}
			
			ps.pluginRouter.CallOnScanCompleted(ps.jobID, libraryID, stats)
		}
	}
}

func (ps *LibraryScanner) preloadFileCache(libraryID uint) error {
	// Load existing media files for this library into cache
	var mediaFiles []database.MediaFile
	err := ps.db.Where("library_id = ?", libraryID).Find(&mediaFiles).Error
	if err != nil {
		return err
	}

	ps.fileCache.mu.Lock()
	defer ps.fileCache.mu.Unlock()

	for _, file := range mediaFiles {
		// Use a copy to avoid pointer issues
		fileCopy := file
		ps.fileCache.cache[file.Path] = &fileCopy
	}

	fmt.Printf("Preloaded %d files into cache\n", len(mediaFiles))
	return nil
}

func (ps *LibraryScanner) countFilesToScan(libraryPath string) (int, error) {
	count := 0
	
	// Get supported extensions from all registered plugins
	var supportedExts map[string]bool
	if ps.corePluginsManager != nil {
		supportedExts = make(map[string]bool)
		plugins := ps.corePluginsManager.GetRegistry().GetPlugins()
		for _, plugin := range plugins {
			for _, ext := range plugin.GetSupportedExtensions() {
				supportedExts[ext] = true
			}
		}
	}
	
	// Fallback to the generic media extensions from utils package if no plugins available
	if len(supportedExts) == 0 {
		supportedExts = utils.MediaExtensions
	}

	err := filepath.WalkDir(libraryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip files/directories we can't access
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if supportedExts[ext] {
			count++
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (ps *LibraryScanner) scanDirectory(libraryID uint) {
	defer ps.wg.Done()

	// Load library to get path
	var library database.MediaLibrary
	if err := ps.db.First(&library, libraryID).Error; err != nil {
		ps.errorQueue <- fmt.Errorf("failed to load library: %w", err)
		return
	}

	// Start with the root directory
	select {
	case <-ps.ctx.Done(): // Check context before attempting to send
		return
	default:
		// Non-blocking send to dirQueue
		select {
		case ps.dirQueue <- dirWork{path: library.Path, libraryID: libraryID}:
		case <-ps.ctx.Done():
			return
		default:
			// This case should ideally not be hit if dirQueue has buffer and dirQueueManager is responsive.
			// If it is, it means the queue is full and context isn't cancelled yet.
			// Depending on desired behavior, could log or handle differently.
			fmt.Printf("Warning: dirQueue full or not ready when initiating scan for library %d\n", libraryID)
			// We might still want to error out or retry, but for now, let the scan proceed (or fail gracefully elsewhere).
		}
	}
}

func (ps *LibraryScanner) worker(id int) {
	defer ps.wg.Done()
	defer ps.activeWorkers.Add(-1)

	ps.activeWorkers.Add(1)
	processedCount := 0

	for {
		select {
		case work, ok := <-ps.workQueue:
			if !ok {
				return
			}

			processedCount++
			if processedCount%1000 == 0 {
				logger.Debug("Worker progress", "worker_id", id, "files_processed", processedCount)
			}

			result := ps.processFile(work)

			select {
			case ps.resultQueue <- result:
			case <-ps.ctx.Done():
				return
			}

		case <-ps.ctx.Done():
			return
		}
	}
}

func (ps *LibraryScanner) processFile(work scanWork) *scanResult {
	path := work.path
	info := work.info
	libraryID := work.libraryID

	// Check if file exists in cache first
	ps.fileCache.mu.RLock()
	cachedFile, exists := ps.fileCache.cache[path]
	ps.fileCache.mu.RUnlock()

	if exists {
		// Check if file has been modified since last scan
		if cachedFile.UpdatedAt.Before(info.ModTime()) || cachedFile.Size != info.Size() {
			// File has changed, process it
		} else {
			// File hasn't changed, skip processing
			ps.filesSkipped.Add(1)
			return &scanResult{path: path}
		}
	}

	// Calculate file hash for duplicate detection
	hash, err := ps.calculateFileHashOptimized(path)
	if err != nil {
		ps.errorsCount.Add(1)
		return &scanResult{
			path:  path,
			error: fmt.Errorf("failed to calculate file hash: %w", err),
		}
	}

	// Create media file record
	mediaFile := &database.MediaFile{
		Path:      path,
		Size:      info.Size(),
		Hash:      hash,
		LibraryID: libraryID,
		ScanJobID: &ps.jobID,
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save MediaFile to database first to get an ID
	if err := ps.db.Create(mediaFile).Error; err != nil {
		ps.errorsCount.Add(1)
		return &scanResult{
			path:  path,
			error: fmt.Errorf("failed to save media file: %w", err),
		}
	}

	// Attempt metadata extraction using the plugin system
	var metadata interface{}
	var metadataType string
	if ps.corePluginsManager != nil {
		// Find a plugin that can handle this file
		corePlugins := ps.corePluginsManager.GetRegistry().GetCorePlugins()
		for _, corePlugin := range corePlugins {
			if corePlugin.Match(path, info) {
				// Create metadata context
				ctx := plugins.MediaContext{
					DB:        ps.db,
					MediaFile: mediaFile,
					LibraryID: libraryID,
					EventBus:  ps.eventBus,
				}
				
				// Use the new HandleFile method that returns MediaItem + assets
				mediaItem, assets, err := corePlugin.HandleFile(path, info, ctx)
				if err != nil {
					// Continue without metadata - not a fatal error
					break
				}
				
				// Save using MediaManager
				if mediaItem != nil {
					if err := ps.mediaManager.SaveMediaItem(mediaItem, assets); err != nil {
						// Continue - this will be handled as a warning
					} else {
						// Extract metadata and type for legacy compatibility
						metadata = mediaItem.Metadata
						metadataType = mediaItem.Type
					}
				}
				break
			}
		}
	}

	ps.filesProcessed.Add(1)
	ps.bytesProcessed.Add(info.Size())

	return &scanResult{
		mediaFile:        mediaFile,
		metadata:         metadata,
		metadataType:     metadataType,
		path:             path,
		needsPluginHooks: true,
	}
}

func (ps *LibraryScanner) calculateFileHashOptimized(path string) (string, error) {
	// Use the standard file hash calculation
	return utils.CalculateFileHash(path)
}

func (ps *LibraryScanner) callPluginHooks(mediaFile *database.MediaFile, metadata interface{}) {
	ps.pluginRouter.CallOnMediaFileScanned(mediaFile, metadata)

	if ps.pluginManager == nil {
		logger.Error("Plugin manager is nil", "file", mediaFile.Path)
		return
	}

	scannerHooks := ps.pluginManager.GetScannerHooks()
	if len(scannerHooks) == 0 {
		return
	}

	metadataMap := make(map[string]string)
	if metadata != nil {
		if musicMeta, ok := metadata.(map[string]interface{}); ok {
			for k, v := range musicMeta {
				metadataMap[k] = fmt.Sprint(v)
			}
		} else if musicMeta, ok := metadata.(*database.MusicMetadata); ok {
			metadataMap["title"] = musicMeta.Title
			metadataMap["artist"] = musicMeta.Artist
			metadataMap["album"] = musicMeta.Album
			metadataMap["album_artist"] = musicMeta.AlbumArtist
			metadataMap["genre"] = musicMeta.Genre
			metadataMap["year"] = fmt.Sprintf("%d", musicMeta.Year)
			metadataMap["track"] = fmt.Sprintf("%d", musicMeta.Track)
			metadataMap["track_total"] = fmt.Sprintf("%d", musicMeta.TrackTotal)
			metadataMap["disc"] = fmt.Sprintf("%d", musicMeta.Disc)
			metadataMap["disc_total"] = fmt.Sprintf("%d", musicMeta.DiscTotal)
			metadataMap["duration"] = fmt.Sprintf("%.0f", musicMeta.Duration)
			metadataMap["bitrate"] = fmt.Sprintf("%d", musicMeta.Bitrate)
			metadataMap["sample_rate"] = fmt.Sprintf("%d", musicMeta.SampleRate)
			metadataMap["channels"] = fmt.Sprintf("%d", musicMeta.Channels)
			metadataMap["format"] = musicMeta.Format
			metadataMap["has_artwork"] = fmt.Sprintf("%t", musicMeta.HasArtwork)
		} else {
			metadataMap["type"] = fmt.Sprintf("%T", metadata)
		}
	}

	for i, hook := range scannerHooks {
		go func(hookIndex int, h proto.ScannerHookServiceClient) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := &proto.OnMediaFileScannedRequest{
				MediaFileId: uint32(mediaFile.ID),
				FilePath:    mediaFile.Path,
				Metadata:    metadataMap,
			}

			if _, err := h.OnMediaFileScanned(ctx, req); err != nil {
				logger.Error("External plugin scanner hook failed",
					"hook_index", hookIndex,
					"file", mediaFile.Path,
					"error", err)
			}
		}(i, hook)
	}
}

func (ps *LibraryScanner) resultProcessor() {
	defer ps.wg.Done()

	for {
		select {
		case result, ok := <-ps.resultQueue:
			if !ok {
				return
			}

			if result.error != nil {
				logger.Error("Scan error", "path", result.path, "error", result.error)
				continue
			}

			if result.mediaFile == nil {
				// Skipped file
				continue
			}

			// Update cache
			ps.fileCache.mu.Lock()
			ps.fileCache.cache[result.path] = result.mediaFile
			ps.fileCache.mu.Unlock()

			// Call plugin hooks if needed
			if result.needsPluginHooks {
				ps.callPluginHooks(result.mediaFile, result.metadata)
			}

		case <-ps.ctx.Done():
			return
		}
	}
}

func (ps *LibraryScanner) batchFlusher() {
	defer ps.wg.Done()

	ticker := time.NewTicker(ps.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ps.dbMutex.Lock()
			if err := ps.batchProcessor.Flush(); err != nil {
				logger.Error("Batch flush failed", "error", err)
			}
			ps.dbMutex.Unlock()
		case <-ps.ctx.Done():
			return
		}
	}
}

func (ps *LibraryScanner) updateProgress() {
	defer ps.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastProcessed := int64(0)

	for {
		select {
		case <-ticker.C:
			processed := ps.filesProcessed.Load()
			skipped := ps.filesSkipped.Load()
			errors := ps.errorsCount.Load()

			// Calculate throughput
			currentProcessed := processed
			throughput := float64(currentProcessed - lastProcessed) // files per second
			lastProcessed = currentProcessed

			// Update progress estimator
			ps.progressEstimator.Update(processed+skipped, ps.bytesProcessed.Load())

			// Get progress info
			progress, eta, rate := ps.progressEstimator.GetEstimate()

			// Get system stats (mock implementation for now)
			cpuPercent := float64(50.0) // Mock value
			memoryMB := float64(1024.0) // Mock value
			diskReadMB := float64(10.0) // Mock value
			diskWriteMB := float64(5.0) // Mock value

			// Get worker stats
			activeWorkers, minWorkers, maxWorkers, queueLen := ps.GetWorkerStats()

			// Emit progress event
			event := events.Event{
				Type:    "scan.progress",
				Source:  "scanner",
				Title:   "Scan Progress",
				Message: fmt.Sprintf("Scanning progress: %d files processed", processed),
				Data: map[string]interface{}{
					"job_id":           ps.jobID,
					"files_processed":  processed,
					"files_skipped":    skipped,
					"errors_count":     errors,
					"throughput_fps":   throughput,
					"progress_percent": progress,
					"eta_time":         eta,
					"processing_rate":  rate,
					"active_workers":   activeWorkers,
					"min_workers":      minWorkers,
					"max_workers":      maxWorkers,
					"queue_length":     queueLen,
					"cpu_percent":      cpuPercent,
					"memory_mb":        memoryMB,
					"disk_read_mb":     diskReadMB,
					"disk_write_mb":    diskWriteMB,
				},
			}
			ps.eventBus.PublishAsync(event)

			// Update scan job in database
			ps.dbMutex.Lock()
			if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Updates(map[string]interface{}{
				"files_processed": processed,
				"bytes_processed": ps.bytesProcessed.Load(),
				"progress":        int(progress),
				"updated_at":      time.Now(),
			}).Error; err != nil {
				logger.Error("Failed to update scan job progress", "error", err)
			}
			ps.dbMutex.Unlock()

		case <-ps.ctx.Done():
			return
		}
	}
}

// Pause pauses the scanning process
func (ps *LibraryScanner) Pause() {
	ps.wasExplicitlyPaused.Store(true)
	ps.cancel()

	ps.dbMutex.Lock()
	defer ps.dbMutex.Unlock()

	if err := ps.db.Model(&database.ScanJob{}).Where("id = ?", ps.jobID).Update("status", "paused").Error; err != nil {
		logger.Error("Failed to update scan job status to paused", "error", err)
	}

	event := events.Event{
		Type:    "scan.paused",
		Source:  "scanner",
		Title:   "Scan Paused",
		Message: fmt.Sprintf("Paused scanning job %d", ps.jobID),
		Data: map[string]interface{}{
			"job_id":          ps.jobID,
			"files_processed": ps.filesProcessed.Load(),
			"files_skipped":   ps.filesSkipped.Load(),
			"errors_count":    ps.errorsCount.Load(),
		},
	}
	ps.eventBus.PublishAsync(event)
}

func (ps *LibraryScanner) adjustWorkers() {
	// Mock system stats for now
	cpuPercent := float64(50.0)
	memoryPercent := float64(60.0)

	currentWorkers := int(ps.activeWorkers.Load())
	queueLen := len(ps.workQueue)

	// Determine target worker count based on system load and queue length
	targetWorkers := currentWorkers

	// Scale up if:
	// - Queue is backing up (more than 50 items per worker)
	// - CPU usage is low (< 70%)
	// - Memory usage is reasonable (< 80%)
	if queueLen > currentWorkers*50 && cpuPercent < 70 && memoryPercent < 80 {
		if currentWorkers < ps.maxWorkers {
			targetWorkers = currentWorkers + 1
		}
	}

	// Scale down if:
	// - Queue is small (< 10 items per worker)
	// - CPU usage is high (> 85%)
	// - Memory usage is high (> 85%)
	if (queueLen < currentWorkers*10 || cpuPercent > 85 || memoryPercent > 85) && currentWorkers > ps.minWorkers {
		targetWorkers = currentWorkers - 1
	}

	// Start new workers if needed
	if targetWorkers > currentWorkers {
		for i := currentWorkers; i < targetWorkers; i++ {
			ps.wg.Add(1)
			go ps.worker(i)
		}
	}

	// Signal workers to exit if needed
	if targetWorkers < currentWorkers {
		exitCount := currentWorkers - targetWorkers
		for i := 0; i < exitCount; i++ {
			select {
			case ps.workerExitChan <- 1:
			default:
				// Channel full, skip
				break
			}
		}
	}
}

func (ps *LibraryScanner) workerPoolManager() {
	defer ps.wg.Done()

	logger.Debug("Starting worker pool")

	// Start initial workers
	for i := 0; i < ps.workerCount; i++ {
		logger.Debug("Starting worker", "worker_id", i)
		ps.wg.Add(1)
		go ps.worker(i)
	}

	logger.Debug("Started initial workers", "count", ps.workerCount)

	// Monitor and adjust worker count
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ps.adjustWorkers()
		case <-ps.ctx.Done():
			logger.Debug("Worker pool manager shutting down")
			return
		}
	}
}

// GetWorkerStats returns current worker statistics
func (ps *LibraryScanner) GetWorkerStats() (active, min, max, queueLen int) {
	return int(ps.activeWorkers.Load()), ps.minWorkers, ps.maxWorkers, len(ps.workQueue)
}

func (bp *BatchProcessor) AddMediaFile(file *database.MediaFile) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.mediaFiles = append(bp.mediaFiles, file)
}

func (bp *BatchProcessor) AddMetadataItem(item MetadataItem) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.metadataItems = append(bp.metadataItems, item)
}

func (bp *BatchProcessor) FlushIfNeeded() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if len(bp.mediaFiles) >= bp.batchSize {
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
	if len(bp.mediaFiles) == 0 && len(bp.metadataItems) == 0 {
		return nil
	}

	// First transaction: Insert media files
	if len(bp.mediaFiles) > 0 {
		err := bp.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(bp.mediaFiles, bp.batchSize).Error; err != nil {
				return fmt.Errorf("failed to insert media files: %w", err)
			}
			return nil
		})
		
		if err != nil {
			logger.Error("Failed to insert media files", "error", err)
			return err
		}
		
		logger.Debug("Media files inserted", "count", len(bp.mediaFiles))
		bp.mediaFiles = bp.mediaFiles[:0]
	}

	// Second transaction: Process metadata items
	if len(bp.metadataItems) > 0 {
		err := bp.db.Transaction(func(tx *gorm.DB) error {
			mediaManager := plugins.NewMediaManager(tx)
			
			for _, item := range bp.metadataItems {
				if item.Data != nil {
					var mediaFile database.MediaFile
					if err := tx.Where("path = ?", item.Path).First(&mediaFile).Error; err != nil {
						logger.Warn("Failed to find media file for metadata",
							"path", item.Path,
							"type", item.Type,
							"error", err)
						continue
					}
					
					mediaItem := &plugins.MediaItem{
						Type:      item.Type,
						MediaFile: &mediaFile,
						Metadata:  item.Data,
					}
					
					if err := mediaManager.SaveMediaItem(mediaItem, []plugins.MediaAsset{}); err != nil {
						logger.Warn("Failed to save metadata",
							"type", item.Type,
							"path", item.Path,
							"error", err)
						continue
					}
					
					logger.Debug("Processed metadata",
						"type", item.Type,
						"path", item.Path)
				}
			}
			return nil
		})
		
		if err != nil {
			logger.Warn("Failed to process metadata items", "error", err)
		}

		logger.Debug("Metadata items processed", "count", len(bp.metadataItems))
		bp.metadataItems = bp.metadataItems[:0]
	}

	return nil
}

func (ps *LibraryScanner) directoryWorker(workerID int) {
	defer ps.wg.Done()
	defer ps.activeDirWorkers.Add(-1)

	ps.activeDirWorkers.Add(1)

	for {
		select {
		case work, ok := <-ps.dirQueue:
			if !ok {
				return
			}

			ps.processDirWork(work)

		case <-ps.ctx.Done():
			return
		}
	}
}

func (ps *LibraryScanner) processDirWork(work dirWork) {
	entries, err := os.ReadDir(work.path)
	if err != nil {
		ps.errorQueue <- fmt.Errorf("failed to read directory %s: %w", work.path, err)
		return
	}

	var supportedExts map[string]bool
	if ps.corePluginsManager != nil {
		supportedExts = make(map[string]bool)
		plugins := ps.corePluginsManager.GetRegistry().GetPlugins()
		for _, plugin := range plugins {
			for _, ext := range plugin.GetSupportedExtensions() {
				supportedExts[ext] = true
			}
		}
	}
	
	if len(supportedExts) == 0 {
		supportedExts = utils.MediaExtensions
	}

	filesQueued := 0
	dirsQueued := 0

	for _, entry := range entries {
		fullPath := filepath.Join(work.path, entry.Name())

		if entry.IsDir() {
			select {
			case ps.dirQueue <- dirWork{path: fullPath, libraryID: work.libraryID}:
				dirsQueued++
			case <-ps.ctx.Done():
				return
			}
		} else {
			ext := filepath.Ext(entry.Name())
			if supportedExts[ext] {
				info, err := entry.Info()
				if err != nil {
					continue
				}

				select {
				case ps.workQueue <- scanWork{
					path:      fullPath,
					info:      info,
					libraryID: work.libraryID,
				}:
					filesQueued++
				case <-ps.ctx.Done():
					return
				}
			}
		}
	}

	if filesQueued > 0 || dirsQueued > 0 {
		logger.Debug("Directory processed",
			"path", work.path,
			"files_queued", filesQueued,
			"dirs_queued", dirsQueued)
	}
}

func (ps *LibraryScanner) dirQueueManager() {
	defer ps.wg.Done()
	defer close(ps.dirQueue)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	consecutiveEmptyChecks := 0

	for {
		select {
		case <-ticker.C:
			activeDirWorkers := ps.activeDirWorkers.Load()
			dirQueueLen := len(ps.dirQueue)

			logger.Debug("Directory queue status",
				"active_workers", activeDirWorkers,
				"queue_length", dirQueueLen)

			if activeDirWorkers == 0 && dirQueueLen == 0 {
				consecutiveEmptyChecks++
				
				if consecutiveEmptyChecks >= 5 {
					logger.Debug("Directory queue manager detected extended idle period")
					
					time.Sleep(1 * time.Second)
					
					if ps.activeDirWorkers.Load() == 0 && len(ps.dirQueue) == 0 {
						logger.Debug("Directory queue manager completed")
						return
					} else {
						logger.Debug("Directory queue manager work resumed")
						consecutiveEmptyChecks = 0
					}
				}
			} else {
				consecutiveEmptyChecks = 0
			}

		case <-ps.ctx.Done():
			logger.Debug("Directory queue manager cancelled")
			return
		}
	}
}

func (ps *LibraryScanner) workQueueCloser() {
	defer ps.wg.Done()
	defer close(ps.workQueue)

	// Monitor directory workers and queue
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if directory scanning is complete
			activeDirWorkers := ps.activeDirWorkers.Load()
			dirQueueLen := len(ps.dirQueue)

			// If no active directory workers and directory queue is empty,
			// we're done adding new work
			if activeDirWorkers == 0 && dirQueueLen == 0 {
				// Wait a bit more to ensure all work is processed
				time.Sleep(2 * time.Second)

				// Final check
				if ps.activeDirWorkers.Load() == 0 && len(ps.dirQueue) == 0 {
					return
				}
			}

		case <-ps.ctx.Done():
			return
		}
	}
}
