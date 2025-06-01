package scanner

import (
	"context"
	"fmt"
	"hash/crc32"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// LibraryScanner implements a high-performance parallel file scanner
type LibraryScanner struct {
	db                *gorm.DB
	eventBus          events.EventBus
	pluginManager     plugins.Manager
	throttler         *AdaptiveThrottler
	telemetry        *ScanTelemetry
	
	// Configuration
	jobID             uint32
	status            string
	
	// Scanning infrastructure
	workerCount       int
	minWorkers        int
	maxWorkers        int
	dirWorkerCount    int
	activeWorkers     atomic.Int32
	activeDirWorkers  atomic.Int32
	
	// Concurrency management
	workerExitChan    chan int
	workQueue         chan scanWork
	dirQueue          chan dirWork
	resultQueue       chan *scanResult
	errorQueue        chan error
	
	// Batch processing
	batchSize         int
	batchTimeout      time.Duration
	batchProcessor    *BatchProcessor
	
	// Context and lifecycle
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	
	// Caching
	fileCache         *FileCache
	metadataCache     *MetadataCache
	
	// Plugin integration
	pluginRouter      *PluginRouter
	corePluginsManager *plugins.CorePluginManager // Changed from interface{} to proper type
	mediaManager      *plugins.MediaManager
	
	// Progress tracking
	progressEstimator *ProgressEstimator
	systemMonitor     *SystemLoadMonitor
	adaptiveThrottler *AdaptiveThrottler
	
	// State management
	scanJob           *database.ScanJob
	startTime         time.Time
	dbMutex           sync.RWMutex
	wasExplicitlyPaused atomic.Bool
	
	// Counters
	filesProcessed    atomic.Int64
	filesFound        atomic.Int64
	filesSkipped      atomic.Int64
	bytesProcessed    atomic.Int64
	bytesFound        atomic.Int64
	errorsCount       atomic.Int64
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

// FileCache provides fast lookups for existing files with bloom filter optimization
type FileCache struct {
	cache map[string]*database.MediaFile
	mu    sync.RWMutex
	// LRU tracking for cache eviction
	accessTimes map[string]time.Time
	maxSize     int
	// Bloom filter for fast pre-screening of known files
	knownFiles  *BloomFilter // Fast membership test for files seen in previous scans
	bloomMu     sync.RWMutex // Separate mutex for bloom filter operations
}

// BloomFilter provides probabilistic membership testing with configurable false positive rate
type BloomFilter struct {
	bitArray []uint64      // Bit array for bloom filter
	size     uint          // Number of bits in the filter
	hashFuncs int          // Number of hash functions to use
	items    uint          // Approximate number of items added
	maxItems uint          // Maximum expected items (for optimal sizing)
}

// NewBloomFilter creates a new bloom filter optimized for file path membership testing
func NewBloomFilter(expectedItems uint, falsePositiveRate float64) *BloomFilter {
	// Calculate optimal size and hash functions
	// For 100k files with 1% false positive rate: ~95KB memory usage
	optimalSize := uint(-float64(expectedItems) * math.Log(falsePositiveRate) / (math.Log(2) * math.Log(2)))
	optimalHashFuncs := int(float64(optimalSize) / float64(expectedItems) * math.Log(2))
	
	// Ensure reasonable bounds
	if optimalHashFuncs < 1 {
		optimalHashFuncs = 1
	}
	if optimalHashFuncs > 7 {
		optimalHashFuncs = 7 // Diminishing returns beyond 7 hash functions
	}
	
	// Round up to nearest 64-bit boundary for efficient operations
	arraySize := (optimalSize + 63) / 64
	
	return &BloomFilter{
		bitArray:  make([]uint64, arraySize),
		size:      arraySize * 64,
		hashFuncs: optimalHashFuncs,
		maxItems:  expectedItems,
	}
}

// Add inserts a file path into the bloom filter
func (bf *BloomFilter) Add(path string) {
	for i := 0; i < bf.hashFuncs; i++ {
		hash := bf.hash(path, uint(i))
		bitIndex := hash % bf.size
		arrayIndex := bitIndex / 64
		bitPosition := bitIndex % 64
		bf.bitArray[arrayIndex] |= 1 << bitPosition
	}
	bf.items++
}

// Contains checks if a file path might be in the set (may have false positives)
func (bf *BloomFilter) Contains(path string) bool {
	for i := 0; i < bf.hashFuncs; i++ {
		hash := bf.hash(path, uint(i))
		bitIndex := hash % bf.size
		arrayIndex := bitIndex / 64
		bitPosition := bitIndex % 64
		if (bf.bitArray[arrayIndex] & (1 << bitPosition)) == 0 {
			return false // Definitely not in set
		}
	}
	return true // Probably in set (could be false positive)
}

// hash computes hash value for a string with a seed
func (bf *BloomFilter) hash(s string, seed uint) uint {
	// Use FNV-1a hash with seed for good distribution
	hash := uint(2166136261) ^ seed // FNV offset basis with seed
	for i := 0; i < len(s); i++ {
		hash ^= uint(s[i])
		hash *= 16777619 // FNV prime
	}
	return hash
}

// EstimatedFalsePositiveRate returns the current estimated false positive rate
func (bf *BloomFilter) EstimatedFalsePositiveRate() float64 {
	if bf.items == 0 {
		return 0
	}
	// Calculate based on actual items added
	bitsPerItem := float64(bf.size) / float64(bf.items)
	return math.Pow(1-math.Exp(-float64(bf.hashFuncs)/bitsPerItem), float64(bf.hashFuncs))
}

// MetadataCache caches metadata extraction results (now generic)
type MetadataCache struct {
	cache map[string]interface{} // Generic metadata cache
	mu    sync.RWMutex
	ttl   time.Duration
	// Cache hit/miss tracking for performance monitoring
	hits   atomic.Int64
	misses atomic.Int64
}

// ThrottleEventHandler implements ThrottleEventCallback to handle throttling events
type ThrottleEventHandler struct {
	scanner  *LibraryScanner
	eventBus events.EventBus
	jobID    uint
}

// OnThrottleAdjustment handles throttling adjustment events
func (teh *ThrottleEventHandler) OnThrottleAdjustment(reason string, oldLimits, newLimits ThrottleLimits, metrics SystemMetrics) {
	if teh.eventBus != nil {
		event := events.Event{
			Type:    "scan.throttle_adjusted",
			Source:  "adaptive_throttler",
			Title:   "Scan Throttling Adjusted",
			Message: fmt.Sprintf("Throttling adjusted: %s", reason),
			Data: map[string]interface{}{
				"job_id":        teh.jobID,
				"reason":        reason,
				"old_workers":   oldLimits.WorkerCount,
				"new_workers":   newLimits.WorkerCount,
				"old_batch":     oldLimits.BatchSize,
				"new_batch":     newLimits.BatchSize,
				"old_delay_ms":  oldLimits.ProcessingDelay.Milliseconds(),
				"new_delay_ms":  newLimits.ProcessingDelay.Milliseconds(),
				"cpu_percent":   metrics.CPUPercent,
				"memory_percent": metrics.MemoryPercent,
				"io_wait_percent": metrics.IOWaitPercent,
				"network_mbps":  metrics.NetworkUtilMBps,
			},
		}
		teh.eventBus.PublishAsync(event)
	}
}

// OnEmergencyBrake handles emergency brake activation
func (teh *ThrottleEventHandler) OnEmergencyBrake(reason string, metrics SystemMetrics) {
	if teh.eventBus != nil {
		event := events.Event{
			Type:    "scan.emergency_brake",
			Source:  "adaptive_throttler",
			Title:   "Emergency Brake Activated",
			Message: fmt.Sprintf("Emergency throttling activated: %s", reason),
			Data: map[string]interface{}{
				"job_id":        teh.jobID,
				"reason":        reason,
				"cpu_percent":   metrics.CPUPercent,
				"memory_percent": metrics.MemoryPercent,
				"io_wait_percent": metrics.IOWaitPercent,
				"load_average":  metrics.LoadAverage,
			},
		}
		teh.eventBus.PublishAsync(event)
	}
}

// OnEmergencyBrakeRelease handles emergency brake release
func (teh *ThrottleEventHandler) OnEmergencyBrakeRelease(metrics SystemMetrics) {
	if teh.eventBus != nil {
		event := events.Event{
			Type:    "scan.emergency_brake_released",
			Source:  "adaptive_throttler",
			Title:   "Emergency Brake Released",
			Message: "Emergency throttling has been released",
			Data: map[string]interface{}{
				"job_id":        teh.jobID,
				"cpu_percent":   metrics.CPUPercent,
				"memory_percent": metrics.MemoryPercent,
				"io_wait_percent": metrics.IOWaitPercent,
				"load_average":  metrics.LoadAverage,
			},
		}
		teh.eventBus.PublishAsync(event)
	}
}

// NewLibraryScanner creates a new parallel file scanner with optimizations
func NewLibraryScanner(db *gorm.DB, jobID uint32, eventBus events.EventBus, pluginManager plugins.Manager) *LibraryScanner {
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate worker counts for maximum performance
	// CRITICAL FIX FOR LARGE VIDEO FILES + SQLITE: 
	// Reduce worker count dramatically to prevent SQLite deadlocks
	// With 618MB average files and 32k+ files, we need fewer workers for stability
	workerCount := runtime.NumCPU() // Use 1x CPU cores instead of 4x for SQLite
	if workerCount < 4 {
		workerCount = 4 // Minimum 4 workers
	}
	if workerCount > 8 {
		workerCount = 8 // Cap at 8 workers for SQLite stability
	}

	// Set up adaptive worker pool parameters - more conservative for SQLite
	minWorkers := 2
	maxWorkers := 6 // Much lower max workers for SQLite + large files
	if maxWorkers > runtime.NumCPU() {
		maxWorkers = runtime.NumCPU()
	}

	// Calculate directory workers for fast directory traversal
	// OPTIMIZED FOR LARGE FILE COLLECTIONS: Fewer directory workers
	dirWorkerCount := runtime.NumCPU() // Use 1x CPU cores instead of 2x
	if dirWorkerCount < 2 {
		dirWorkerCount = 2 // Minimum 2 directory workers
	}
	if dirWorkerCount > 6 {
		dirWorkerCount = 6 // Cap at 6 directory workers
	}

	// OPTIMIZED QUEUE SIZES: Larger queues for massive media libraries
	workQueueBuffer := workerCount * 2000     // Increased buffer size for large libraries (handles ~24k files)
	dirQueueBuffer := dirWorkerCount * 10000  // Larger directory buffer for deep directory trees

	scanner := &LibraryScanner{
		db:             db,
		eventBus:       eventBus,
		pluginManager:  pluginManager,
		throttler:      NewAdaptiveThrottler(ThrottleConfig{
			MinWorkers:              minWorkers,
			MaxWorkers:              maxWorkers,
			InitialWorkers:          workerCount,
			TargetCPUPercent:        75.0, // Higher CPU target for better performance
			MaxCPUPercent:           90.0, // Higher CPU limit for processing large files
			TargetMemoryPercent:     70.0, // Conservative memory for variable file sizes
			MaxMemoryPercent:        85.0, // Safe memory limit for large batches
			TargetNetworkThroughput: 90.0,  // Reasonable network target for various file sizes
			MaxNetworkThroughput:    130.0, // Conservative network limit for large transfers
			MaxIOWaitPercent:        60.0,  // Allow higher I/O wait for large files
			TargetIOWaitPercent:     40.0,  // Balanced I/O wait target
			MinBatchSize:            1,     // Allow single-file processing for huge files
			MaxBatchSize:            10,    // Reasonable max batch for small files
			DefaultBatchSize:        5,     // Balanced default batch size
			MinProcessingDelay:      0,     // No minimum delay for performance
			MaxProcessingDelay:      75 * time.Millisecond, // Brief delays for system stability
			DefaultProcessingDelay:  8 * time.Millisecond,  // Minimal processing delay
			AdjustmentInterval:      18 * time.Second, // Reasonable adjustment frequency
			EmergencyBrakeThreshold: 98.0,   // Much higher emergency threshold (was 95.0)
			EmergencyBrakeDuration:  4 * time.Second, // Brief emergency brake for recovery
			DNSTimeoutMs:            1800,   // Reasonable DNS timeout for various conditions
			NetworkHealthCheckURL:   "8.8.8.8:53",
		}),
		telemetry: NewScanTelemetry(eventBus, uint(jobID)),
		jobID:    jobID,
		status:   "running",
		workerCount:    workerCount,
		minWorkers:     minWorkers,
		maxWorkers:     maxWorkers,
		dirWorkerCount: dirWorkerCount,
		
		workerExitChan: make(chan int, maxWorkers),
		workQueue:      make(chan scanWork, workQueueBuffer),   // Library-specific work queue
		dirQueue:       make(chan dirWork, dirQueueBuffer),     // Library-specific directory queue
		resultQueue:    make(chan *scanResult, workerCount*10),
		errorQueue:     make(chan error, workerCount),
		batchSize:      500, // Process files in batches of 500 for high performance
		batchTimeout:   5 * time.Second,
		ctx:            ctx,
		cancel:         cancel,
		fileCache: &FileCache{
			cache:       make(map[string]*database.MediaFile),
			mu:          sync.RWMutex{},
			accessTimes: make(map[string]time.Time),
			maxSize:     workerCount * 1000, // 1000 files per worker
			// Initialize bloom filter for ~100k files with 1% false positive rate
			// This uses only ~95KB memory but dramatically speeds up rescans
			knownFiles:  NewBloomFilter(100000, 0.01),
		},
	}

	// Initialize plugin integration components
	scanner.pluginRouter = NewPluginRouter(pluginManager)
	
	// Initialize core plugins manager
	// Note: Core plugins are now managed by the main plugin manager
	// This scanner will integrate with the plugin system when available
	scanner.corePluginsManager = nil // Will be set by plugin integration

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

	// Initialize adaptive throttler with INTELLIGENT configuration
	// Dynamically adapt to workload characteristics rather than assuming content types
	throttleConfig := ThrottleConfig{
		MinWorkers:              minWorkers,
		MaxWorkers:              maxWorkers,
		InitialWorkers:          workerCount,
		TargetCPUPercent:        75.0, // Higher CPU target for better performance
		MaxCPUPercent:           90.0, // Higher CPU limit for processing large files
		TargetMemoryPercent:     70.0, // Conservative memory for variable file sizes
		MaxMemoryPercent:        85.0, // Safe memory limit for large batches
		TargetNetworkThroughput: 90.0,  // Reasonable network target for various file sizes
		MaxNetworkThroughput:    130.0, // Conservative network limit for large transfers
		MaxIOWaitPercent:        60.0,  // Allow higher I/O wait for large files
		TargetIOWaitPercent:     40.0,  // Balanced I/O wait target
		MinBatchSize:            1,     // Allow single-file processing for huge files
		MaxBatchSize:            10,    // Reasonable max batch for small files
		DefaultBatchSize:        5,     // Balanced default batch size
		MinProcessingDelay:      0,     // No minimum delay for performance
		MaxProcessingDelay:      75 * time.Millisecond, // Brief delays for system stability
		DefaultProcessingDelay:  8 * time.Millisecond,  // Minimal processing delay
		AdjustmentInterval:      18 * time.Second, // Reasonable adjustment frequency
		EmergencyBrakeThreshold: 98.0,   // Much higher emergency threshold (was 95.0)
		EmergencyBrakeDuration:  4 * time.Second, // Brief emergency brake for recovery
		DNSTimeoutMs:            1800,   // Reasonable DNS timeout for various conditions
		NetworkHealthCheckURL:   "8.8.8.8:53",
	}
	scanner.adaptiveThrottler = NewAdaptiveThrottler(throttleConfig)

	// Register throttle event handler
	throttleHandler := &ThrottleEventHandler{
		scanner:  scanner,
		eventBus: eventBus,
		jobID:    uint(jobID),
	}
	scanner.adaptiveThrottler.RegisterEventCallback(throttleHandler)

	// Start the adaptive throttler
	if err := scanner.adaptiveThrottler.Start(); err != nil {
		logger.Warn("Failed to start adaptive throttler", "error", err)
	}

	return scanner
}

// Start begins the parallel scanning process
func (ps *LibraryScanner) Start(libraryID uint32) error {
	logger.Debug("LibraryScanner.Start called", "library_id", libraryID)

	// Load library
	var library database.MediaLibrary
	if err := ps.db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("failed to load library: %w", err)
	}

	if library.Path == "" {
		return fmt.Errorf("library path is empty")
	}

	logger.Debug("Library loaded successfully", "library_id", libraryID, "path", library.Path)

	// Create scan job
	ps.scanJob = &database.ScanJob{
		LibraryID: libraryID,
		Status:    "running",
		StartedAt: &time.Time{},
	}
	*ps.scanJob.StartedAt = time.Now()

	if err := ps.db.Create(ps.scanJob).Error; err != nil {
		return fmt.Errorf("failed to create scan job: %w", err)
	}

	ps.jobID = ps.scanJob.ID
	logger.Debug("Scan job created", "job_id", ps.jobID)

	// Emit scan started event
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

	// Initialize state
	ps.startTime = time.Now()
	ps.filesProcessed.Store(0)
	ps.bytesProcessed.Store(0)
	ps.errorsCount.Store(0)
	ps.filesSkipped.Store(0)
	ps.filesFound.Store(0)
	ps.bytesFound.Store(0)

	logger.Debug("Scanner state initialized")

	if err := ps.preloadFileCache(uint(libraryID)); err != nil {
		logger.Warn("Failed to preload file cache", "error", err)
	}

	fileCount, err := ps.countFilesToScan(library.Path)
	if err != nil {
		logger.Warn("Failed to count files for progress estimation", "error", err)
		fileCount = 1000
	}
	ps.progressEstimator.SetTotal(int64(fileCount), 0)

	logger.Debug("About to start all workers and processors")

	// Start all workers and processors
	ps.wg.Add(1)
	go ps.workerPoolManager()
	logger.Debug("Started workerPoolManager goroutine")

	ps.wg.Add(1)
	go ps.resultProcessor()
	logger.Debug("Started resultProcessor goroutine")

	ps.wg.Add(1)
	go ps.batchFlusher()
	logger.Debug("Started batchFlusher goroutine")

	ps.wg.Add(1)
	go ps.dirQueueManager()
	logger.Debug("Started dirQueueManager goroutine")

	ps.wg.Add(1)
	go ps.workQueueCloser()
	logger.Debug("Started workQueueCloser goroutine")

	ps.wg.Add(1)
	go ps.updateProgress()
	logger.Debug("Started updateProgress goroutine")

	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
		logger.Debug("Started directory worker", "worker_id", i)
	}

	ps.wg.Add(1)
	go ps.scanDirectory(uint(libraryID))
	logger.Debug("Started scanDirectory goroutine")

	logger.Debug("All goroutines started, waiting for completion")

	ps.wg.Wait()

	logger.Debug("All goroutines completed, updating final status")

	ps.updateFinalStatus()

	return nil
}

// Resume resumes a paused scan job
func (ps *LibraryScanner) Resume(libraryID uint32) error {
	logger.Debug("LibraryScanner.Resume called", "library_id", libraryID)

	// Load scan job
	if err := ps.loadScanJob(); err != nil {
		return fmt.Errorf("failed to load scan job: %w", err)
	}

	logger.Debug("Scan job loaded", "job_id", ps.jobID, "status", ps.scanJob.Status)

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

	logger.Debug("Library loaded for resume", "library_id", libraryID, "path", library.Path)

	// Update job status to running
	ps.scanJob.Status = "running"
	resumeTime := time.Now()
	ps.scanJob.StartedAt = &resumeTime // Reset started time for resume
	if err := ps.db.Save(ps.scanJob).Error; err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}

	logger.Debug("Scan job status updated to running")

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

	// FIXED: Initialize resume state properly with existing progress
	ps.startTime = resumeTime
	
	// Restore progress from database instead of resetting to zero
	existingFilesProcessed := int64(ps.scanJob.FilesProcessed)
	existingBytesProcessed := ps.scanJob.BytesProcessed
	existingFilesFound := int64(ps.scanJob.FilesFound)
	
	// Set atomic counters to existing progress
	ps.filesProcessed.Store(existingFilesProcessed)
	ps.bytesProcessed.Store(existingBytesProcessed)
	ps.filesFound.Store(existingFilesFound)
	ps.bytesFound.Store(0) // Reset bytes found for discovery during resume
	
	// Reset error counters (these should start fresh)
	ps.errorsCount.Store(0)
	ps.filesSkipped.Store(0)

	logger.Debug("Resume state initialized", "existing_files_processed", existingFilesProcessed, "existing_bytes_processed", existingBytesProcessed, "existing_files_found", existingFilesFound)

	// Preload file cache for performance
	if err := ps.preloadFileCache(uint(libraryID)); err != nil {
		// Log warning but continue
		fmt.Printf("Warning: Failed to preload file cache: %v\n", err)
	}

	// FIXED: Count remaining files and initialize progress estimator properly
	totalFileCount, err := ps.countFilesToScan(library.Path)
	if err != nil {
		fmt.Printf("Warning: Failed to count files for progress estimation: %v\n", err)
		// Use existing found count as fallback if we have it
		if existingFilesFound > 0 {
			totalFileCount = int(existingFilesFound)
		} else {
			totalFileCount = 1000 // Last resort fallback
		}
	}
	
	// FIXED: Initialize progress estimator with total files and current progress
	// Create a fresh progress estimator for the resume operation
	ps.progressEstimator = NewProgressEstimator()
	ps.progressEstimator.SetTotal(int64(totalFileCount), 0)
	
	// FIXED: Initialize progress estimator with existing progress data
	if existingFilesProcessed > 0 {
		ps.progressEstimator.Update(existingFilesProcessed, existingBytesProcessed)
	}
	
	fmt.Printf("INFO: Resuming scan - Total files: %d, Already processed: %d, Remaining: %d\n", 
		totalFileCount, existingFilesProcessed, totalFileCount-int(existingFilesProcessed))

	logger.Debug("About to start all workers and processors for resume")

	// Start the worker pool
	ps.wg.Add(1)
	go ps.workerPoolManager()
	logger.Debug("Started workerPoolManager goroutine for resume")

	// Start result processor
	ps.wg.Add(1)
	go ps.resultProcessor()
	logger.Debug("Started resultProcessor goroutine for resume")

	// Start batch flusher
	ps.wg.Add(1)
	go ps.batchFlusher()
	logger.Debug("Started batchFlusher goroutine for resume")

	// Start queue managers
	ps.wg.Add(1)
	go ps.dirQueueManager()
	logger.Debug("Started dirQueueManager goroutine for resume")

	ps.wg.Add(1)
	go ps.workQueueCloser()
	logger.Debug("Started workQueueCloser goroutine for resume")

	// Start progress updates
	ps.wg.Add(1)
	go ps.updateProgress()
	logger.Debug("Started updateProgress goroutine for resume")

	// Start directory workers
	for i := 0; i < ps.dirWorkerCount; i++ {
		ps.wg.Add(1)
		go ps.directoryWorker(i)
		logger.Debug("Started directory worker for resume", "worker_id", i)
	}

	// Kick off the scanning process
	ps.wg.Add(1)
	go ps.scanDirectory(uint(libraryID))
	logger.Debug("Started scanDirectory goroutine for resume")

	logger.Debug("All goroutines started for resume, waiting for completion")

	// Wait for completion or cancellation
	ps.wg.Wait()

	logger.Debug("All goroutines completed for resume, updating final status")

	// Update final status
	ps.updateFinalStatus()

	return nil
}

func (ps *LibraryScanner) loadScanJob() error {
	return ps.db.First(&ps.scanJob, ps.jobID).Error
}

func (ps *LibraryScanner) updateFinalStatus() {
	// Stop adaptive throttler
	if ps.adaptiveThrottler != nil {
		ps.adaptiveThrottler.Stop()
	}

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
			"files_found":     int(ps.filesFound.Load()),
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
			ps.scanJob.FilesFound = int(ps.filesFound.Load())
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
				"files_found":     ps.filesFound.Load(),
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
			libraryID := uint32(0)
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
			
			ps.pluginRouter.CallOnScanCompleted(uint(ps.jobID), uint(libraryID), stats)
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
	ps.fileCache.bloomMu.Lock()
	defer ps.fileCache.mu.Unlock()
	defer ps.fileCache.bloomMu.Unlock()

	for _, file := range mediaFiles {
		// Use a copy to avoid pointer issues
		fileCopy := file
		ps.fileCache.cache[file.Path] = &fileCopy
		
		// Add to bloom filter for fast future lookups
		ps.fileCache.knownFiles.Add(file.Path)
	}

	fmt.Printf("Preloaded %d files into cache and bloom filter (%.2f%% estimated false positive rate)\n", 
		len(mediaFiles), ps.fileCache.knownFiles.EstimatedFalsePositiveRate()*100)
	return nil
}

func (ps *LibraryScanner) countFilesToScan(libraryPath string) (int, error) {
	count := 0
	
	// Get supported extensions from all registered plugins
	var supportedExts map[string]bool
	if ps.corePluginsManager != nil {
		supportedExts = make(map[string]bool)
		handlers := ps.corePluginsManager.GetEnabledFileHandlers()
		for _, handler := range handlers {
			for _, ext := range handler.GetSupportedExtensions() {
				supportedExts[ext] = true
			}
		}
		logger.Debug("Using core plugins extensions", "path", libraryPath, "ext_count", len(supportedExts))
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

	logger.Debug("Starting directory scan", "library_id", libraryID, "path", library.Path)

	// Start with the root directory
	select {
	case <-ps.ctx.Done(): // Check context before attempting to send
		logger.Debug("Context cancelled before starting directory scan", "library_id", libraryID)
		return
	default:
		// Blocking send to dirQueue with context cancellation
		select {
		case ps.dirQueue <- dirWork{path: library.Path, libraryID: libraryID}:
			logger.Debug("Successfully queued root directory for scanning", "library_id", libraryID, "path", library.Path)
		case <-ps.ctx.Done():
			logger.Debug("Context cancelled while queueing root directory", "library_id", libraryID)
			return
		}
	}
}

func (ps *LibraryScanner) worker(id int) {
	defer ps.wg.Done()
	defer ps.activeWorkers.Add(-1)

	ps.activeWorkers.Add(1)
	processedCount := 0

	logger.Debug("Worker started", "worker_id", id, "active_workers", ps.activeWorkers.Load())

	for {
		select {
		case work, ok := <-ps.workQueue:
			if !ok {
				logger.Debug("Worker exiting - work queue closed", "worker_id", id)
				return
			}

			// Apply intelligent adaptive throttling for optimal system performance
			if ps.adaptiveThrottler != nil {
				ps.adaptiveThrottler.ApplyDelay()
			}

			processedCount++
			if processedCount%1000 == 0 {
				logger.Debug("Worker progress", "worker_id", id, "files_processed", processedCount)
			}

			result := ps.processFile(work)

			select {
			case ps.resultQueue <- result:
				// Successful processing - no logging needed for performance
			case <-ps.ctx.Done():
				logger.Debug("Worker cancelled while sending result", "worker_id", id)
				return
			}

		case <-ps.ctx.Done():
			logger.Debug("Worker cancelled", "worker_id", id)
			return
		}
	}
}

func (ps *LibraryScanner) processFile(work scanWork) *scanResult {
	path := work.path
	info := work.info
	libraryID := work.libraryID

	// Check context at the start of processing
	select {
	case <-ps.ctx.Done():
		return &scanResult{path: path, error: fmt.Errorf("scan cancelled")}
	default:
	}

	// Check context before expensive operations
	select {
	case <-ps.ctx.Done():
		return &scanResult{path: path, error: fmt.Errorf("scan cancelled")}
	default:
	}

	// Calculate file hash with optimized strategy
	hash, err := ps.calculateFileHashOptimized(path)
	if err != nil {
			ps.errorsCount.Add(1)
			return &scanResult{
				path:  path,
				error: fmt.Errorf("failed to calculate file hash: %w", err),
			}
	}

	// Check context before database operations
	select {
	case <-ps.ctx.Done():
		return &scanResult{path: path, error: fmt.Errorf("scan cancelled")}
	default:
	}

	// Create media file record and save to database immediately to get an ID
	mediaFile := &database.MediaFile{
		ID:        uuid.New().String(), // Generate UUID for the ID field
		Path:      path,
		SizeBytes: info.Size(),
		Hash:      hash,
		LibraryID: uint32(libraryID),
		ScanJobID: &ps.jobID,
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save MediaFile to database to get a valid ID for metadata foreign key
	if err := ps.db.Create(mediaFile).Error; err != nil {
		ps.errorsCount.Add(1)
		return &scanResult{
			path:  path,
			error: fmt.Errorf("failed to save media file: %w", err),
		}
	}

	// Update counters immediately for progress tracking
	ps.filesProcessed.Add(1)
	ps.bytesProcessed.Add(info.Size())

	// Extract metadata using available file handler plugins
	var extractedMetadata interface{}
	var metadataType string = "file"
	
	if ps.pluginManager != nil {
		// Get all available file handlers (core and external plugins)
		handlers := ps.pluginManager.GetFileHandlers()
		
		// Allow multiple plugins to handle the same file (e.g., FFmpeg for technical data + enrichment for music metadata)
		var pluginsProcessed []string
		for _, handler := range handlers {
			if handler.Match(path, info) {
				// Create metadata context for the plugin
				ctx := plugins.MetadataContext{
					DB:        ps.db,
					MediaFile: mediaFile, // Now has a valid ID
					LibraryID: libraryID,
					EventBus:  ps.eventBus,
				}
				
				// Extract metadata using the plugin
				if err := handler.HandleFile(path, ctx); err != nil {
					logger.Debug("Plugin metadata extraction failed", "plugin", handler.GetName(), "file", path, "error", err)
					// Don't fail the whole scan for metadata extraction issues
				} else {
					logger.Debug("Successfully extracted metadata", "plugin", handler.GetName(), "file", path)
					pluginsProcessed = append(pluginsProcessed, handler.GetName())
					
					// Mark as successfully processed if any plugin succeeded
					extractedMetadata = map[string]interface{}{"processed": true, "plugins": pluginsProcessed}
					metadataType = "music" // This will be overridden to match the primary content type
					logger.Debug("Metadata extraction completed", "plugin", handler.GetName(), "file", path)
				}
				// Continue to allow other plugins to also process this file
			}
		}
	}

	return &scanResult{
		mediaFile:        mediaFile,
		metadata:         extractedMetadata, // Now contains actual metadata!
		
		metadataType:     metadataType,      // "music" for successful extraction, "file" for basic
		path:             path,
		needsPluginHooks: true, // Enable plugin hooks for enrichment
	}
}

func (ps *LibraryScanner) calculateFileHashOptimized(path string) (string, error) {
	// GENERIC FILE SIZE OPTIMIZATION:
	// Use different hashing strategies based purely on file size characteristics
	// This adapts to any media type without making content assumptions
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	
	// Size-based thresholds for optimal performance
	const tinyFileThreshold = 10 * 1024 * 1024       // 10MB - full hash for small files
	const smallFileThreshold = 100 * 1024 * 1024     // 100MB - full hash for reasonable files  
	const mediumFileThreshold = 1024 * 1024 * 1024   // 1GB - sampled hash for larger files
	const largeFileThreshold = 5 * 1024 * 1024 * 1024 // 5GB - aggressive sampling
	const hugeFileThreshold = 20 * 1024 * 1024 * 1024 // 20GB - ultra-fast sampling
	
	fileSize := fileInfo.Size()
	sizeMB := float64(fileSize) / (1024 * 1024)
	sizeGB := sizeMB / 1024
	
	if fileSize <= tinyFileThreshold {
		// Tiny files (metadata, images, short clips): full hashing for accuracy
		return utils.CalculateFileHash(path)
	} else if fileSize <= smallFileThreshold {
		// Small files (compressed episodes, music): full hashing is still fast
		logger.Debug("Using full hash for small file", "path", path, "size_mb", int64(sizeMB))
		return utils.CalculateFileHash(path)
	} else if fileSize <= mediumFileThreshold {
		// Medium files (HD content): standard sampled hashing
		logger.Debug("Using sampled hash for medium file", "path", path, "size_mb", int64(sizeMB))
		return utils.CalculateFileHashSampled(path, fileSize)
	} else if fileSize <= largeFileThreshold {
		// Large files (4K content, remux): aggressive sampled hashing
		logger.Debug("Using sampled hash for large file", "path", path, "size_gb", float64(int64(sizeGB*10))/10)
		return utils.CalculateFileHashSampled(path, fileSize)
	} else if fileSize <= hugeFileThreshold {
		// Very large files (high bitrate 4K): ultra-fast sampling
		logger.Debug("Using ultra-fast hash for very large file", "path", path, "size_gb", float64(int64(sizeGB*10))/10)
		return utils.CalculateFileHashUltraFast(path, fileSize)
	} else {
		// Massive files (uncompressed, raw): minimal sampling for speed
		logger.Debug("Using ultra-fast hash for massive file", "path", path, "size_gb", float64(int64(sizeGB*10))/10)
		return utils.CalculateFileHashUltraFast(path, fileSize)
	}
}

// uuidToUint32 converts a UUID string to uint32 for plugin compatibility
// This is a temporary solution until plugins are updated to support string UUIDs
func uuidToUint32(uuidStr string) uint32 {
	// Use CRC32 hash of the UUID string to create a deterministic uint32
	hash := crc32.ChecksumIEEE([]byte(uuidStr))
	return hash
}

func (ps *LibraryScanner) callPluginHooks(mediaFile *database.MediaFile, metadata interface{}) {
	if ps.pluginManager == nil {
		return
	}

	// Get scanner hooks from plugin manager
	scannerHooks := ps.pluginManager.GetScannerHooks()
	if len(scannerHooks) == 0 {
		return
	}

	// Convert metadata to string map for plugins
	metadataMap := make(map[string]string)
	if metadata != nil {
		// Extract metadata from database for external plugins
		metadataMap = ps.extractMetadataForPlugins(mediaFile)
	}

	for i, hook := range scannerHooks {
		logger.Debug("Processing scanner hook", "hook_index", i, "file", mediaFile.Path)
		go func(hookIndex int, h proto.ScannerHookServiceClient) {
			logger.Debug("Starting gRPC call to scanner hook", "hook_index", hookIndex, "file", mediaFile.Path)
			// Use the scanner's context with a timeout, so hooks are cancelled when scan is terminated
			ctx, cancel := context.WithTimeout(ps.ctx, 30*time.Second)
			defer cancel()

			req := &proto.OnMediaFileScannedRequest{
				MediaFileId: mediaFile.ID, // Pass the original UUID string directly
				FilePath:    mediaFile.Path,
				Metadata:    metadataMap,
			}

			if _, err := h.OnMediaFileScanned(ctx, req); err != nil {
				logger.Error("External plugin scanner hook failed",
					"hook_index", hookIndex,
					"file", mediaFile.Path,
					"error", err)
			} else {
				logger.Debug("Scanner hook completed successfully", "hook_index", hookIndex, "file", mediaFile.Path)
			}
		}(i, hook)
	}
}

// extractMetadataForPlugins extracts metadata from the database for external plugins
func (ps *LibraryScanner) extractMetadataForPlugins(mediaFile *database.MediaFile) map[string]string {
	metadataMap := make(map[string]string)
	
	// If the media file has a media_id and media_type, extract metadata from the appropriate table
	if mediaFile.MediaID != "" && mediaFile.MediaType != "" {
		switch mediaFile.MediaType {
		case "track":
			// Extract track metadata
			var track database.Track
			if err := ps.db.Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
				metadataMap["title"] = track.Title
				metadataMap["track_number"] = fmt.Sprintf("%d", track.TrackNumber)
				metadataMap["duration"] = fmt.Sprintf("%d", track.Duration)
				
				// Get artist information
				var artist database.Artist
				if err := ps.db.Where("id = ?", track.ArtistID).First(&artist).Error; err == nil {
					metadataMap["artist"] = artist.Name
				}
				
				// Get album information
				var album database.Album
				if err := ps.db.Where("id = ?", track.AlbumID).First(&album).Error; err == nil {
					metadataMap["album"] = album.Title
					if album.ReleaseDate != nil {
						metadataMap["year"] = fmt.Sprintf("%d", album.ReleaseDate.Year())
					}
				}
			}
		case "album":
			// Extract album metadata
			var album database.Album
			if err := ps.db.Where("id = ?", mediaFile.MediaID).First(&album).Error; err == nil {
				metadataMap["album"] = album.Title
				if album.ReleaseDate != nil {
					metadataMap["year"] = fmt.Sprintf("%d", album.ReleaseDate.Year())
				}
				
				// Get artist information
				var artist database.Artist
				if err := ps.db.Where("id = ?", album.ArtistID).First(&artist).Error; err == nil {
					metadataMap["artist"] = artist.Name
				}
			}
		case "artist":
			// Extract artist metadata
			var artist database.Artist
			if err := ps.db.Where("id = ?", mediaFile.MediaID).First(&artist).Error; err == nil {
				metadataMap["artist"] = artist.Name
			}
		}
	}
	
	// If no metadata was found from database, try to extract from filename
	if len(metadataMap) == 0 {
		metadataMap = ps.extractMetadataFromFilename(mediaFile.Path)
	}
	
	logger.Debug("Extracted metadata for external plugins", "file", mediaFile.Path, "metadata_count", len(metadataMap))
	return metadataMap
}

// extractMetadataFromFilename extracts basic metadata from filename as fallback
func (ps *LibraryScanner) extractMetadataFromFilename(filePath string) map[string]string {
	metadataMap := make(map[string]string)
	
	// Extract filename without extension
	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	if ext != "" {
		filename = filename[:len(filename)-len(ext)]
	}
	
	// Try to parse common patterns like "Artist - Album - Track - Title"
	parts := strings.Split(filename, " - ")
	if len(parts) >= 2 {
		// Pattern: "Artist - Title" or "Artist - Album - Title"
		metadataMap["artist"] = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			metadataMap["title"] = strings.TrimSpace(parts[1])
		} else if len(parts) >= 3 {
			metadataMap["album"] = strings.TrimSpace(parts[1])
			metadataMap["title"] = strings.TrimSpace(parts[len(parts)-1])
		}
		
		// Try to extract track number if present
		for i, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) <= 3 && isNumeric(part) {
				metadataMap["track_number"] = part
				// Remove track number from title if it was included
				if i == len(parts)-2 && len(parts) > 2 {
					metadataMap["title"] = strings.TrimSpace(parts[len(parts)-1])
				}
				break
			}
		}
	} else {
		// Fallback: use filename as title
		metadataMap["title"] = filename
	}
	
	// Extract directory-based metadata (artist/album from path)
	pathParts := strings.Split(filepath.Dir(filePath), string(filepath.Separator))
	if len(pathParts) >= 2 {
		// Common pattern: /music/Artist/Album/Track.ext
		if metadataMap["artist"] == "" && len(pathParts) >= 2 {
			artistDir := pathParts[len(pathParts)-2]
			if artistDir != "" && artistDir != "." {
				metadataMap["artist"] = artistDir
			}
		}
		if metadataMap["album"] == "" && len(pathParts) >= 1 {
			albumDir := pathParts[len(pathParts)-1]
			if albumDir != "" && albumDir != "." {
				metadataMap["album"] = albumDir
			}
		}
	}
	
	return metadataMap
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
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

			// Route all media files through batch processor for optimized database operations
			if result.metadataType == "file" {
				ps.batchProcessor.AddMediaFile(result.mediaFile)
				
				// Trigger flush if needed to maintain reasonable batch sizes
				if err := ps.batchProcessor.FlushIfNeeded(); err != nil {
					logger.Error("Batch flush failed during result processing", "error", err, "path", result.path)
				}
			}

			// MediaFiles are already saved in processFile, just add to cache
			// Only process metadata if it was extracted
			if result.metadataType == "music" && result.metadata != nil {
				// Music metadata was successfully extracted and saved
				logger.Debug("Music metadata processed", "file", result.path)
			}
			
			// Update cache after successful processing
			ps.fileCache.Set(result.path, result.mediaFile)

			// Call plugin hooks for enrichment and additional processing
			if result.needsPluginHooks {
				logger.Debug("Calling plugin hooks", "file", result.mediaFile.Path, "needs_hooks", result.needsPluginHooks)
				
				// Call external plugin hooks (gRPC)
				ps.callPluginHooks(result.mediaFile, result.metadata)
				
				// Call internal plugin hooks (direct Go interface)
				if ps.pluginRouter != nil {
					ps.pluginRouter.CallOnMediaFileScanned(result.mediaFile, result.metadata)
				}
			}

		case <-ps.ctx.Done():
			return
		}
	}
}

func (ps *LibraryScanner) batchFlusher() {
	defer ps.wg.Done()

	// Aggressive flushing optimized for large file collections
	// Frequent DB commits prevent memory buildup and ensure data persistence
	ticker := time.NewTicker(2 * time.Second) // Every 2 seconds for optimal performance
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
			// Force flush all remaining items before shutdown
			ps.dbMutex.Lock()
			if err := ps.batchProcessor.Flush(); err != nil {
				logger.Error("Final batch flush failed", "error", err)
			}
			ps.dbMutex.Unlock()
			return
		}
	}
}

func (ps *LibraryScanner) updateProgress() {
	defer ps.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastFilesProcessed := int64(0)
	lastBytesProcessed := int64(0)
	lastUpdateTime := time.Now()

	for {
		select {
		case <-ticker.C:
			currentFiles := ps.filesProcessed.Load()
			currentBytes := ps.bytesProcessed.Load()
			currentErrors := ps.errorsCount.Load()
			currentSkipped := ps.filesSkipped.Load()
			currentFound := ps.filesFound.Load()
			currentBytesFound := ps.bytesFound.Load()
			now := time.Now()

			// CRITICAL FIX: Update progress estimator with current progress
			// This is essential for ETA calculation in GetDetailedScanProgress API
			if ps.progressEstimator != nil {
				// Update progress estimator with current totals and progress
				ps.progressEstimator.SetTotal(currentFound, currentBytes)
				ps.progressEstimator.Update(currentFiles, currentBytes)
			}

			// Calculate rates
			elapsedSinceLastUpdate := now.Sub(lastUpdateTime).Seconds()
			filesPerSecond := float64(currentFiles-lastFilesProcessed) / elapsedSinceLastUpdate
			mbPerSecond := float64(currentBytes-lastBytesProcessed) / (1024 * 1024) / elapsedSinceLastUpdate

			// Calculate overall progress
			var progress float64
			if currentFound > 0 {
				progress = float64(currentFiles) / float64(currentFound) * 100
			}

			// Get adaptive throttling metrics
			var throttleLimits ThrottleLimits
			var systemMetrics SystemMetrics  
			var networkStats NetworkStats
			var throttleConfig ThrottleConfig
			var emergencyBrakeActive bool
			
			if ps.adaptiveThrottler != nil {
				throttleLimits = ps.adaptiveThrottler.GetCurrentLimits()
				systemMetrics = ps.adaptiveThrottler.GetSystemMetrics()
				networkStats = ps.adaptiveThrottler.GetNetworkStats()
				throttleConfig = ps.adaptiveThrottler.GetThrottleConfig()

				// Check if emergency brake is active
				shouldThrottle, _ := ps.adaptiveThrottler.ShouldThrottle()
				emergencyBrakeActive = shouldThrottle
			} else {
				// Default values when throttler is not available
				throttleLimits = ThrottleLimits{BatchSize: 5, WorkerCount: 8}
				emergencyBrakeActive = false
			}

			// Cache performance metrics
			cacheHitRate := ps.metadataCache.GetHitRate()
			cacheSize := len(ps.fileCache.cache)

			// Calculate ETA
			var eta time.Time
			if filesPerSecond > 0 && currentFound > currentFiles {
				remainingFiles := currentFound - currentFiles
				secondsRemaining := float64(remainingFiles) / filesPerSecond
				eta = now.Add(time.Duration(secondsRemaining) * time.Second)
			}

			// Update database
			ps.dbMutex.Lock()
			ps.scanJob.FilesProcessed = int(currentFiles)
			ps.scanJob.FilesFound = int(currentFound)
			ps.scanJob.FilesSkipped = int(currentSkipped)
			ps.scanJob.BytesProcessed = currentBytes
			ps.scanJob.Progress = progress
			ps.scanJob.UpdatedAt = now
			if err := ps.db.Save(ps.scanJob).Error; err != nil {
				logger.Error("Failed to update scan job progress", "error", err)
			}
			ps.dbMutex.Unlock()

			// Create enhanced progress event with throttling data
			event := events.Event{
				Type:    "scan.progress",
				Source:  "scanner",
				Title:   "Scan Progress",
				Message: fmt.Sprintf("Scanning in progress: %.1f%% complete", progress),
				Data: map[string]interface{}{
					"jobId":           ps.jobID,
					"filesProcessed":  currentFiles,
					"filesFound":      currentFound,
					"filesSkipped":    currentSkipped,
					"bytesProcessed":  currentBytes,
					"errorsCount":     currentErrors,
					"progress":        progress,
					"activeWorkers":   int(ps.activeWorkers.Load()),
					"queueDepth":      len(ps.workQueue),
					"filesPerSecond":  filesPerSecond,
					"throughputMbps":  mbPerSecond,
					
					// Additional UI-friendly fields
					"totalFiles":      currentFound, // Alias for filesFound
					"totalBytes":      currentBytesFound, // Total bytes discovered
					"remainingFiles":  currentFound - currentFiles,
					
					"throttling": map[string]interface{}{
						"enabled":               throttleLimits.Enabled,
						"batchSize":             throttleLimits.BatchSize,
						"processingDelayMs":     throttleLimits.ProcessingDelay.Milliseconds(),
						"networkBandwidthLimit": throttleLimits.NetworkBandwidth,
						"ioThrottlePercent":     throttleLimits.IOThrottle,
						"emergencyBrakeActive":  emergencyBrakeActive,
					},

					"systemMetrics": map[string]interface{}{
						"cpuPercent":       systemMetrics.CPUPercent,
						"memoryPercent":    systemMetrics.MemoryPercent,
						"memoryUsedMB":     systemMetrics.MemoryUsedMB,
						"ioWaitPercent":    systemMetrics.IOWaitPercent,
						"loadAverage":      systemMetrics.LoadAverage,
						"networkMbps":      systemMetrics.NetworkUtilMBps,
						"diskReadMbps":     systemMetrics.DiskReadMBps,
						"diskWriteMbps":    systemMetrics.DiskWriteMBps,
						"metricsTimestamp": systemMetrics.TimestampUTC,
					},

					"throttleConfig": map[string]interface{}{
						"targetCpuPercent":        throttleConfig.TargetCPUPercent,
						"maxCpuPercent":           throttleConfig.MaxCPUPercent,
						"targetMemoryPercent":     throttleConfig.TargetMemoryPercent,
						"maxMemoryPercent":        throttleConfig.MaxMemoryPercent,
						"targetNetworkMbps":       throttleConfig.TargetNetworkThroughput,
						"maxNetworkMbps":          throttleConfig.MaxNetworkThroughput,
						"emergencyBrakeThreshold": throttleConfig.EmergencyBrakeThreshold,
					},
					
					"networkHealth": map[string]interface{}{
						"dnsLatencyMs":      networkStats.DNSLatencyMs,
						"networkLatencyMs":  networkStats.NetworkLatencyMs,
						"packetLossPercent": networkStats.PacketLossPercent,
						"connectionErrors":  networkStats.ConnectionErrors,
						"isHealthy":         networkStats.IsHealthy,
						"lastHealthCheck":   networkStats.LastHealthCheck,
					},
					
					// Cache performance metrics
					"cacheMetrics": map[string]interface{}{
						"hitRate":     cacheHitRate,
						"cacheSize":   cacheSize,
						"maxSize":     ps.fileCache.maxSize,
					},
				},
			}

			// Add ETA if available
			if !eta.IsZero() {
				event.Data["eta"] = eta
				event.Data["estimatedTimeLeft"] = eta.Sub(now).Seconds()
			}

			ps.eventBus.PublishAsync(event)

			// Log detailed progress for monitoring
			logger.Info("Scan progress update",
				"job_id", ps.jobID,
				"files_processed", currentFiles,
				"files_found", currentFound,
				"progress_percent", fmt.Sprintf("%.1f", progress),
				"files_per_second", fmt.Sprintf("%.1f", filesPerSecond),
				"throughput_mbps", fmt.Sprintf("%.1f", mbPerSecond),
				"active_workers", int(ps.activeWorkers.Load()),
				"queue_depth", len(ps.workQueue),
				"cpu_percent", fmt.Sprintf("%.1f", systemMetrics.CPUPercent),
				"memory_percent", fmt.Sprintf("%.1f", systemMetrics.MemoryPercent),
				"processing_delay_ms", throttleLimits.ProcessingDelay.Milliseconds(),
				"emergency_brake", emergencyBrakeActive,
			)

			// Log warning if performance is degraded
			if emergencyBrakeActive {
				logger.Warn("Emergency brake active - scan performance degraded",
					"reason", "High system load",
					"cpu_percent", systemMetrics.CPUPercent,
					"memory_percent", systemMetrics.MemoryPercent,
					"io_wait_percent", systemMetrics.IOWaitPercent,
				)
			}

			// Log network issues if detected
			if !networkStats.IsHealthy {
				logger.Warn("Network health issues detected",
					"dns_latency_ms", networkStats.DNSLatencyMs,
					"packet_loss_percent", networkStats.PacketLossPercent,
					"connection_errors", networkStats.ConnectionErrors,
				)
			}

			// Update for next iteration
			lastFilesProcessed = currentFiles
			lastBytesProcessed = currentBytes
			lastUpdateTime = now

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
			"files_found":     ps.filesFound.Load(),
			"files_processed": ps.filesProcessed.Load(),
			"files_skipped":   ps.filesSkipped.Load(),
			"errors_count":    ps.errorsCount.Load(),
		},
	}
	ps.eventBus.PublishAsync(event)
}

func (ps *LibraryScanner) adjustWorkers() {
	// Use adaptive throttler for intelligent worker scaling
	if ps.adaptiveThrottler == nil {
		return
	}
	
	throttleLimits := ps.adaptiveThrottler.GetCurrentLimits()
	
	currentWorkers := int(ps.activeWorkers.Load())
	targetWorkers := throttleLimits.WorkerCount
	
	// Update batch size based on throttler recommendations
	ps.batchSize = throttleLimits.BatchSize
	if ps.batchProcessor != nil {
		ps.batchProcessor.batchSize = throttleLimits.BatchSize
	}

	// Start new workers if needed
	if targetWorkers > currentWorkers {
		for i := currentWorkers; i < targetWorkers; i++ {
			ps.wg.Add(1)
			go ps.worker(i)
		}
		logger.Debug("Scaled up workers", 
			"from", currentWorkers, 
			"to", targetWorkers,
			"batch_size", ps.batchSize)
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
		logger.Debug("Scaled down workers", 
			"from", currentWorkers, 
			"to", targetWorkers,
			"batch_size", ps.batchSize)
	}
}

func (ps *LibraryScanner) workerPoolManager() {
	defer ps.wg.Done()

	logger.Debug("Starting worker pool manager")

	// Start initial workers
	logger.Debug("About to start initial workers", "count", ps.workerCount)
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

	// INTELLIGENT BATCH SIZING: Analyze current batch characteristics
	// Adapt batch size based on file sizes and memory impact rather than content assumptions
	
	batchSize := 5 // Default batch size for typical files
	totalBatchSize := int64(0)
	maxFileSize := int64(0)
	
	// Analyze current batch characteristics
	for _, mediaFile := range bp.mediaFiles {
		totalBatchSize += mediaFile.SizeBytes
		if mediaFile.SizeBytes > maxFileSize {
			maxFileSize = mediaFile.SizeBytes
		}
	}
	
	// Size-based batch optimization thresholds
	const smallFileSize = 100 * 1024 * 1024      // 100MB
	const largeFileSize = 2 * 1024 * 1024 * 1024 // 2GB  
	const hugeFileSize = 10 * 1024 * 1024 * 1024 // 10GB
	const massiveFileSize = 50 * 1024 * 1024 * 1024 // 50GB
	
	// Adaptive batch sizing based on largest file in batch
	if maxFileSize >= massiveFileSize {
		// Massive files (50GB+): immediate processing to prevent memory issues
		batchSize = 1
		logger.Debug("Using immediate flush for massive files", "max_file_gb", float64(maxFileSize)/(1024*1024*1024), "batch_count", len(bp.mediaFiles))
	} else if maxFileSize >= hugeFileSize {
		// Huge files (10-50GB): very small batches
		batchSize = 2
		logger.Debug("Using small batch for huge files", "max_file_gb", float64(maxFileSize)/(1024*1024*1024), "batch_count", len(bp.mediaFiles))
	} else if maxFileSize >= largeFileSize {
		// Large files (2-10GB): small batches
		batchSize = 3
	} else if maxFileSize >= smallFileSize {
		// Medium files (100MB-2GB): standard batches
		batchSize = 5
	} else {
		// Small files (<100MB): larger batches for efficiency
		batchSize = 10
	}
	
	// Also consider total batch memory footprint
	const maxBatchMemory = 5 * 1024 * 1024 * 1024 // 5GB total batch size limit
	if totalBatchSize > maxBatchMemory {
		logger.Debug("Using immediate flush for large batch memory", "total_batch_gb", float64(totalBatchSize)/(1024*1024*1024))
		return bp.flushInternal()
	}

	if len(bp.mediaFiles) >= batchSize {
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

	// Use a single large transaction for optimal SQLite performance
	// This dramatically reduces locking overhead and ensures atomicity
	err := bp.db.Transaction(func(tx *gorm.DB) error {
		
		// Process media files with UPSERT logic using SQLite's ON CONFLICT
		if len(bp.mediaFiles) > 0 {
			// For SQLite, we'll use UPSERT operations with ON CONFLICT DO UPDATE
			// This handles both new files and updates to existing files atomically
			for _, mediaFile := range bp.mediaFiles {
				// Use GORM's Save method which handles UPSERT automatically
				// For SQLite, this will use INSERT OR REPLACE or ON CONFLICT UPDATE
				if err := tx.Save(mediaFile).Error; err != nil {
					logger.Debug("Failed to save media file", "path", mediaFile.Path, "error", err)
					// Continue processing other files instead of failing the whole batch
					continue
				}
			}
			
			logger.Debug("Batch processed media files with UPSERT", "count", len(bp.mediaFiles))
		}

		// Process metadata items in the same transaction
		if len(bp.metadataItems) > 0 {
			mediaManager := plugins.NewMediaManager(tx)
			
			for _, item := range bp.metadataItems {
				if item.Data != nil {
					var mediaFile database.MediaFile
					if err := tx.Where("path = ?", item.Path).First(&mediaFile).Error; err != nil {
						logger.Debug("Skipping metadata for missing file", "path", item.Path)
						continue
					}
					
					mediaItem := &plugins.MediaItem{
						Type:      item.Type,
						MediaFile: &mediaFile,
						Metadata:  item.Data,
					}
					
					if err := mediaManager.SaveMediaItem(mediaItem, []plugins.MediaAsset{}); err != nil {
						logger.Debug("Failed to save metadata", "path", item.Path, "error", err)
						continue
					}
				}
			}
			
			logger.Debug("Processed metadata items", "count", len(bp.metadataItems))
		}

		return nil
	})
	
	if err != nil {
		logger.Error("Batch flush transaction failed", "error", err, "media_files", len(bp.mediaFiles), "metadata_items", len(bp.metadataItems))
		return err
	}

	// Clear the batches after successful transaction
	bp.mediaFiles = bp.mediaFiles[:0]
	bp.metadataItems = bp.metadataItems[:0]

	logger.Debug("Batch flush completed successfully with UPSERT operations")
	return nil
}

func (ps *LibraryScanner) directoryWorker(workerID int) {
	defer ps.wg.Done()
	defer ps.activeDirWorkers.Add(-1)

	ps.activeDirWorkers.Add(1)
	logger.Debug("Directory worker started", "worker_id", workerID)

	for {
		select {
		case work, ok := <-ps.dirQueue:
			if !ok {
				logger.Debug("Directory worker exiting - queue closed", "worker_id", workerID)
				return
			}

			logger.Debug("Directory worker processing", "worker_id", workerID, "path", work.path, "library_id", work.libraryID)
			ps.processDirWork(work)

		case <-ps.ctx.Done():
			logger.Debug("Directory worker cancelled", "worker_id", workerID)
			return
		}
	}
}

func (ps *LibraryScanner) processDirWork(work dirWork) {
	// NFS-optimized directory reading with error resilience
	entries, err := os.ReadDir(work.path)
	if err != nil {
		// NFS can have temporary issues, retry once before failing
		time.Sleep(100 * time.Millisecond)
		entries, err = os.ReadDir(work.path)
		if err != nil {
			ps.errorQueue <- fmt.Errorf("failed to read directory %s: %w", work.path, err)
			return
		}
	}

	// Pre-filter and batch process directory entries
	var supportedExts map[string]bool
	if ps.corePluginsManager != nil {
		supportedExts = make(map[string]bool)
		handlers := ps.corePluginsManager.GetEnabledFileHandlers()
		for _, handler := range handlers {
			for _, ext := range handler.GetSupportedExtensions() {
				supportedExts[ext] = true
			}
		}
		logger.Debug("Using core plugins extensions", "path", work.path, "ext_count", len(supportedExts))
	}
	
	if len(supportedExts) == 0 {
		supportedExts = utils.MediaExtensions
		logger.Debug("Using fallback MediaExtensions", "path", work.path, "ext_count", len(supportedExts))
	}

	filesQueued := 0
	dirsQueued := 0
	filesFoundInThisDir := 0
	skippedDirs := 0
	skippedFiles := 0

	// Smart directory filtering for performance
	knownSkipDirs := map[string]bool{
		".git": true, ".svn": true, ".hg": true,
		"node_modules": true, ".DS_Store": true,
		"@eaDir": true, ".@__thumb": true, // Synology system dirs
		"#recycle": true, ".recycle": true, // Recycle bins
		"System Volume Information": true, // Windows system
		".Trash": true, ".Trashes": true, // macOS trash
		"lost+found": true, // Linux filesystem
		"$RECYCLE.BIN": true, // Windows recycle bin
		
		// Media server specific directories
		"Plex Versions": true, // Plex optimized versions
		"Optimized for TV": true, // Plex TV optimizations
		".plex": true, // Plex metadata
		".emby": true, // Emby metadata
		".jellyfin": true, // Jellyfin metadata
		"metadata": true, // Generic metadata dirs
		"cache": true, // Cache directories
		"tmp": true, "temp": true, // Temporary directories
		
		// Trickplay and preview directories (multiple patterns)
		"trickplay": true, "Trickplay": true, "TRICKPLAY": true,
		"previews": true, "Previews": true, "PREVIEWS": true,
		"thumbnails": true, "Thumbnails": true, "THUMBNAILS": true,
		"sprites": true, "Sprites": true, "SPRITES": true,
		"chapters": true, "Chapters": true, "CHAPTERS": true,
		"keyframes": true, "Keyframes": true, "KEYFRAMES": true,
		"timeline": true, "Timeline": true, "TIMELINE": true,
		"storyboard": true, "Storyboard": true, "STORYBOARD": true,
	}
	
	// Enhanced trickplay directory detection patterns
	trickplayPatterns := []string{
		"trickplay", "preview", "thumbnail", "sprite", "chapter",
		"keyframe", "timeline", "storyboard", "scene", "frame",
		"bif", "vtt", "cache", "temp", "meta",
	}

	// Batch file operations for NFS efficiency
	mediaFiles := make([]fs.DirEntry, 0, len(entries))
	subdirs := make([]fs.DirEntry, 0, len(entries))

	// First pass: categorize entries efficiently
	for _, entry := range entries {
		entryName := entry.Name()
		
		// Skip hidden files and known system directories
		if strings.HasPrefix(entryName, ".") && knownSkipDirs[entryName] {
			skippedFiles++
			continue
		}
		
		if entry.IsDir() {
			// Skip known system/cache directories
			if knownSkipDirs[entryName] {
				skippedDirs++
				continue
			}
			// Skip trickplay directories entirely using enhanced pattern matching
			entryNameLower := strings.ToLower(entryName)
			isTrickplayDir := false
			for _, pattern := range trickplayPatterns {
				if strings.Contains(entryNameLower, pattern) {
					isTrickplayDir = true
					break
				}
			}
			if isTrickplayDir {
				skippedDirs++
				continue
			}
			// Skip very deep nested directories (potential infinite loops)
			if strings.Count(work.path, string(filepath.Separator)) > 50 {
				skippedDirs++
				continue
			}
			subdirs = append(subdirs, entry)
		} else {
			// ENHANCED TRICKPLAY FILTERING: Apply multiple layers of filtering during discovery
			fullPath := filepath.Join(work.path, entryName)
			
			// Layer 1: Quick extension check for media files FIRST
			ext := strings.ToLower(filepath.Ext(entryName))
			if !supportedExts[ext] {
				skippedFiles++
				continue
			}
			
			// Layer 2: Skip files that are explicitly marked to be skipped (trickplay, subtitles, etc.)
			if utils.IsSkippedFile(fullPath) {
				skippedFiles++
				continue
			}
			
			// Layer 3: Enhanced trickplay file detection using patterns
			if utils.IsTrickplayFile(fullPath) {
				skippedFiles++
				continue
			}
			
			// Layer 4: Skip trickplay files by filename patterns (additional safety net)
			entryNameLower := strings.ToLower(entryName)
			isTrickplayFile := false
			for _, pattern := range trickplayPatterns {
				if strings.Contains(entryNameLower, pattern) {
					isTrickplayFile = true
					break
				}
			}
			if isTrickplayFile {
				skippedFiles++
				continue
			}
			
			// Layer 5: Skip files in parent directories that suggest trickplay content
			parentDir := strings.ToLower(filepath.Base(work.path))
			isInTrickplayDir := false
			for _, pattern := range trickplayPatterns {
				if strings.Contains(parentDir, pattern) {
					isInTrickplayDir = true
					break
				}
			}
			if isInTrickplayDir {
				skippedFiles++
				continue
			}
			
			// If we reach here, it's a valid media file that should be processed
			mediaFiles = append(mediaFiles, entry)
		}
	}

	// Batch file info operations for NFS
	// Process media files in batches to reduce NFS round trips
	const batchSize = 25 // Smaller batches for NFS optimization
	
	for i := 0; i < len(mediaFiles); i += batchSize {
		end := i + batchSize
		if end > len(mediaFiles) {
			end = len(mediaFiles)
		}
		
		// Process this batch of files
		for j := i; j < end; j++ {
			entry := mediaFiles[j]
			fullPath := filepath.Join(work.path, entry.Name())
			
			// Bloom filter pre-screening for ultra-fast rescan performance
			// Stage 1: Check bloom filter first (< 1μs, no I/O)
			ps.fileCache.bloomMu.RLock()
			likelyKnown := ps.fileCache.knownFiles.Contains(fullPath)
			ps.fileCache.bloomMu.RUnlock()
			
			if likelyKnown {
				// Stage 2: File is probably in our database, check cache (fast, ~10μs)
				if cachedFile, exists := ps.fileCache.Get(fullPath); exists {
					// Stage 3: Quick metadata check to see if file changed (NFS I/O)
					if info, err := entry.Info(); err == nil {
						if cachedFile.SizeBytes == info.Size() && !cachedFile.UpdatedAt.Before(info.ModTime()) {
							// File hasn't changed, skip processing entirely
							skippedFiles++
							continue
						}
					}
				}
				// Note: If bloom filter says "probably known" but cache miss,
				// it could be a false positive or cache eviction - continue processing
			}
			// If bloom filter says "definitely unknown", always process the file
			
			info, err := entry.Info()
			if err != nil {
				logger.Debug("Failed to get file info", "path", fullPath, "error", err)
				continue
			}

			// Skip very small files that are likely incomplete or metadata
			if info.Size() < 1024 {
				skippedFiles++
				continue
			}

			// Increment files found counter immediately when we discover a supported file
			ps.filesFound.Add(1)
			ps.bytesFound.Add(info.Size()) // Track total bytes discovered
			filesFoundInThisDir++

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
		
		// Small delay between batches to avoid overwhelming NFS
		if i+batchSize < len(mediaFiles) {
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Queue subdirectories for processing
	for _, entry := range subdirs {
		fullPath := filepath.Join(work.path, entry.Name())
		select {
		case ps.dirQueue <- dirWork{path: fullPath, libraryID: work.libraryID}:
			dirsQueued++
		case <-ps.ctx.Done():
			return
		}
	}

	// Immediately update progress estimator total if we found significant files
	if filesFoundInThisDir > 0 {
		// Don't constantly update progress estimator total - causes unstable progress
		// Instead, just emit discovery events for UI feedback
		
		// Emit immediate discovery event for responsive UI feedback
		event := events.Event{
			Type:    "scan.discovery",
			Source:  "scanner",
			Title:   "Files Discovered",
			Message: fmt.Sprintf("Discovered %d new files in %s", filesFoundInThisDir, work.path),
			Data: map[string]interface{}{
				"job_id":              ps.jobID,
				"directory":           work.path,
				"files_found_in_dir":  filesFoundInThisDir,
				"total_files_found":   ps.filesFound.Load(),
				"discovery_phase":     true, // Indicate this is discovery, not final progress
				"dirs_skipped":        skippedDirs,
				"files_skipped":       skippedFiles,
			},
		}
		ps.eventBus.PublishAsync(event)
	}

	// SAFEGUARD: Worker queue monitoring to prevent starvation
	totalFilesFound := ps.filesFound.Load()
	totalFilesProcessed := ps.filesProcessed.Load()
	currentQueueDepth := len(ps.workQueue)
	
	// Check for potential queue starvation (files found but not being processed)
	if totalFilesFound > 50 && totalFilesProcessed == 0 && currentQueueDepth == 0 && filesQueued == 0 {
		logger.Warn("Worker queue starvation detected",
			"job_id", ps.jobID,
			"files_found", totalFilesFound,
			"files_processed", totalFilesProcessed,
			"queue_depth", currentQueueDepth,
			"files_queued_this_dir", filesQueued,
			"directory", work.path,
			"recommendation", "Check filtering logic - files are being discovered but not queued for processing")
		
		// Emit warning event for monitoring systems
		warningEvent := events.Event{
			Type:    "scan.queue_starvation_warning",
			Source:  "scanner",
			Title:   "Worker Queue Starvation Detected",
			Message: fmt.Sprintf("Scan job %d has found %d files but none are being processed", ps.jobID, totalFilesFound),
			Data: map[string]interface{}{
				"job_id":              ps.jobID,
				"files_found":         totalFilesFound,
				"files_processed":     totalFilesProcessed,
				"queue_depth":         currentQueueDepth,
				"active_workers":      ps.activeWorkers.Load(),
				"issue_type":          "queue_starvation",
				"severity":            "warning",
			},
		}
		ps.eventBus.PublishAsync(warningEvent)
		
		// FALLBACK: Try to queue at least a few files with minimal filtering
		logger.Warn("Activating fallback processing mode", "job_id", ps.jobID, "directory", work.path)
		fallbackCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if supportedExts[ext] && fallbackCount < 5 {
				if info, err := entry.Info(); err == nil && info.Size() > 1024 {
					fullPath := filepath.Join(work.path, entry.Name())
					select {
					case ps.workQueue <- scanWork{path: fullPath, info: info, libraryID: work.libraryID}:
						fallbackCount++
						filesQueued++
						ps.filesFound.Add(1)
					case <-ps.ctx.Done():
						return
					}
				}
			}
		}
		if fallbackCount > 0 {
			logger.Info("Fallback queued files", "count", fallbackCount, "directory", work.path)
		}
	}

	if filesQueued > 0 || dirsQueued > 0 || skippedFiles > 0 || skippedDirs > 0 {
		logger.Debug("Directory processed",
			"path", work.path,
			"files_queued", filesQueued,
			"files_found", filesFoundInThisDir,
			"dirs_queued", dirsQueued,
			"files_skipped", skippedFiles,
			"dirs_skipped", skippedDirs,
			"total_entries", len(entries))
	}
}

func (ps *LibraryScanner) dirQueueManager() {
	defer ps.wg.Done()
	defer close(ps.dirQueue)

	logger.Debug("Directory queue manager started")

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
						
						// Directory discovery is now complete - set final total for stable progress
						finalTotal := ps.filesFound.Load()
						finalBytes := ps.bytesFound.Load()
						if finalTotal > 0 {
							ps.progressEstimator.SetTotal(finalTotal, finalBytes)
							ps.progressEstimator.SetDiscoveryComplete()
							
							// Emit discovery completion event
							event := events.Event{
								Type:    "scan.discovery_complete",
								Source:  "scanner",
								Title:   "File Discovery Complete",
								Message: fmt.Sprintf("Discovery complete: found %d files to process", finalTotal),
								Data: map[string]interface{}{
									"job_id":              ps.jobID,
									"final_total_files":   finalTotal,
									"final_total_bytes":   finalBytes,
									"discovery_complete":  true,
								},
							}
							ps.eventBus.PublishAsync(event)
							
							logger.Info("File discovery completed", 
								"job_id", ps.jobID, 
								"total_files_found", finalTotal,
								"total_bytes_found", finalBytes)
						}
						
						return
					} else {
						logger.Debug("Directory queue manager work resumed")
						
						// Reset consecutive empty checks
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

// Cache management methods for performance monitoring
func (fc *FileCache) Get(path string) (*database.MediaFile, bool) {
	fc.mu.RLock()
	file, exists := fc.cache[path]
	fc.mu.RUnlock()
	
	// Update access time with proper write lock if file exists
	// Use sampling to reduce lock contention - only update 10% of the time
	if exists && shouldUpdateAccessTime() {
		fc.mu.Lock()
		// Double-check the entry still exists after acquiring write lock
		if _, stillExists := fc.cache[path]; stillExists {
			fc.accessTimes[path] = time.Now()
		}
		fc.mu.Unlock()
	}
	
	return file, exists
}

// shouldUpdateAccessTime uses sampling to reduce access time update frequency
// This significantly reduces lock contention while maintaining LRU effectiveness
func shouldUpdateAccessTime() bool {
	// Update access time for only ~10% of requests to reduce lock contention
	// This is sufficient for LRU tracking while improving performance
	return time.Now().UnixNano()%10 == 0
}

func (fc *FileCache) Set(path string, file *database.MediaFile) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	
	// Check if we need to evict old entries
	if len(fc.cache) >= fc.maxSize {
		fc.evictOldestLRU()
	}
	
	fc.cache[path] = file
	fc.accessTimes[path] = time.Now()
	
	// Add to bloom filter for future fast lookups
	fc.bloomMu.Lock()
	fc.knownFiles.Add(path)
	fc.bloomMu.Unlock()
}

func (fc *FileCache) evictOldestLRU() {
	// Find the oldest accessed file
	var oldestPath string
	var oldestTime time.Time = time.Now()
	
	for path, accessTime := range fc.accessTimes {
		if accessTime.Before(oldestTime) {
			oldestTime = accessTime
			oldestPath = path
		}
	}
	
	if oldestPath != "" {
		delete(fc.cache, oldestPath)
		delete(fc.accessTimes, oldestPath)
	}
}

func (mc *MetadataCache) Get(path string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	metadata, exists := mc.cache[path]
	if exists {
		mc.hits.Add(1)
	} else {
		mc.misses.Add(1)
	}
	return metadata, exists
}

func (mc *MetadataCache) GetHitRate() float64 {
	hits := mc.hits.Load()
	misses := mc.misses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// convertSecsToTimeCode converts duration in seconds to MM:SS format
func convertSecsToTimeCode(secs float64) string {
	minutes := int(secs) / 60
	seconds := int(secs) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}


