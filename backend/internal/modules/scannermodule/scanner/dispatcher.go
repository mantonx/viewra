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
)

// WorkDispatcher manages work distribution and worker pools
type WorkDispatcher struct {
	// Worker management
	workerCount    int
	minWorkers     int
	maxWorkers     int
	activeWorkers  atomic.Int32
	workerExitChan chan int
	
	// Directory workers
	dirWorkerCount   int
	activeDirWorkers atomic.Int32
	
	// Queues
	workQueue   chan scanWork
	dirQueue    chan dirWork
	resultQueue chan *scanResult
	errorQueue  chan error
	
	// State management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Configuration
	config *ScanConfig
}

// NewWorkDispatcher creates a new work dispatcher
func NewWorkDispatcher(config *ScanConfig) *WorkDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Calculate optimal worker count based on CPU cores
	workerCount := config.WorkerCount
	if workerCount == 0 {
		workerCount = runtime.NumCPU()
		if workerCount < 2 {
			workerCount = 2
		} else if workerCount > 8 {
			workerCount = 8 // Cap at 8 workers to avoid overwhelming the system
		}
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
	
	return &WorkDispatcher{
		workerCount:      workerCount,
		minWorkers:       minWorkers,
		maxWorkers:       maxWorkers,
		dirWorkerCount:   dirWorkerCount,
		workerExitChan:   make(chan int, maxWorkers),
		workQueue:        make(chan scanWork, config.ChannelBufferSize),
		dirQueue:         make(chan dirWork, config.ChannelBufferSize*5),
		resultQueue:      make(chan *scanResult, config.ChannelBufferSize/10),
		errorQueue:       make(chan error, workerCount),
		ctx:              ctx,
		cancel:           cancel,
		config:           config,
	}
}

// Start initializes and starts the worker pool
func (wd *WorkDispatcher) Start() error {
	// Start initial workers
	for i := 0; i < wd.workerCount; i++ {
		wd.wg.Add(1)
		go wd.worker(i)
	}
	
	// Start directory workers
	for i := 0; i < wd.dirWorkerCount; i++ {
		wd.wg.Add(1)
		go wd.directoryWorker(i)
	}
	
	// Start worker pool manager for dynamic scaling
	wd.wg.Add(1)
	go wd.workerPoolManager()
	
	// Start queue managers
	wd.wg.Add(1)
	go wd.dirQueueManager()
	
	wd.wg.Add(1)
	go wd.workQueueCloser()
	
	return nil
}

// Stop gracefully shuts down the dispatcher
func (wd *WorkDispatcher) Stop() {
	wd.cancel()
	wd.wg.Wait()
}

// EnqueueDirectory adds a directory to be scanned
func (wd *WorkDispatcher) EnqueueDirectory(path string, libraryID uint) error {
	select {
	case wd.dirQueue <- dirWork{path: path, libraryID: libraryID}:
		return nil
	case <-wd.ctx.Done():
		return fmt.Errorf("dispatcher context cancelled")
	default:
		return fmt.Errorf("directory queue is full")
	}
}

// GetWorkQueue returns the work queue for external processing
func (wd *WorkDispatcher) GetWorkQueue() <-chan scanWork {
	return wd.workQueue
}

// GetResultQueue returns the result queue for external processing
func (wd *WorkDispatcher) GetResultQueue() <-chan *scanResult {
	return wd.resultQueue
}

// GetErrorQueue returns the error queue for external processing
func (wd *WorkDispatcher) GetErrorQueue() <-chan error {
	return wd.errorQueue
}

// GetStats returns current dispatcher statistics
func (wd *WorkDispatcher) GetStats() WorkerPoolStats {
	return WorkerPoolStats{
		ActiveWorkers:   int(wd.activeWorkers.Load()),
		TotalWorkers:    wd.workerCount,
		QueueLength:     len(wd.workQueue),
		ProcessingRate:  0, // This would be calculated by the caller
		AverageWaitTime: 0, // This would be calculated by the caller
	}
}

// worker processes files from the work queue
func (wd *WorkDispatcher) worker(id int) {
	defer wd.wg.Done()
	defer wd.activeWorkers.Add(-1)
	
	wd.activeWorkers.Add(1)
	
	for {
		select {
		case work, ok := <-wd.workQueue:
			if !ok {
				return
			}
			
			// Process the work (this would be handled by the scanner)
			// For now, we just pass it to the result queue
			result := &scanResult{
				path: work.path,
				// Processing would happen here
			}
			
			select {
			case wd.resultQueue <- result:
			case <-wd.ctx.Done():
				return
			}
			
		case <-wd.ctx.Done():
			return
		}
	}
}

// directoryWorker processes directories from the directory queue
func (wd *WorkDispatcher) directoryWorker(workerID int) {
	defer wd.wg.Done()
	defer wd.activeDirWorkers.Add(-1)
	
	wd.activeDirWorkers.Add(1)
	
	for {
		select {
		case work, ok := <-wd.dirQueue:
			if !ok {
				return
			}
			
			wd.processDirWork(work)
			
		case <-wd.ctx.Done():
			return
		}
	}
}

// processDirWork scans a directory and enqueues files and subdirectories
func (wd *WorkDispatcher) processDirWork(work dirWork) {
	entries, err := os.ReadDir(work.path)
	if err != nil {
		select {
		case wd.errorQueue <- fmt.Errorf("failed to read directory %s: %w", work.path, err):
		default:
			// Error queue full, skip
		}
		return
	}
	
	supportedExts := map[string]bool{
		".mp3":  true,
		".flac": true,
		".wav":  true,
		".m4a":  true,
		".aac":  true,
		".ogg":  true,
		".wma":  true,
		".opus": true,
	}
	
	for _, entry := range entries {
		fullPath := filepath.Join(work.path, entry.Name())
		
		if entry.IsDir() {
			// Queue subdirectory for processing
			select {
			case wd.dirQueue <- dirWork{path: fullPath, libraryID: work.libraryID}:
			case <-wd.ctx.Done():
				return
			}
		} else {
			// Check if it's a supported audio file
			ext := filepath.Ext(entry.Name())
			if supportedExts[ext] {
				info, err := entry.Info()
				if err != nil {
					continue // Skip files we can't stat
				}
				
				// Queue file for scanning
				select {
				case wd.workQueue <- scanWork{
					path:      fullPath,
					info:      info,
					libraryID: work.libraryID,
				}:
				case <-wd.ctx.Done():
					return
				}
			}
		}
	}
}

// workerPoolManager monitors and adjusts worker count based on load
func (wd *WorkDispatcher) workerPoolManager() {
	defer wd.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			wd.adjustWorkers()
		case <-wd.ctx.Done():
			return
		}
	}
}

// adjustWorkers dynamically adjusts the number of workers based on queue load
func (wd *WorkDispatcher) adjustWorkers() {
	// Mock system stats for now
	cpuPercent := float64(50.0)
	memoryPercent := float64(60.0)
	
	currentWorkers := int(wd.activeWorkers.Load())
	queueLen := len(wd.workQueue)
	
	// Determine target worker count based on system load and queue length
	targetWorkers := currentWorkers
	
	// Scale up if:
	// - Queue is backing up (more than 50 items per worker)
	// - CPU usage is low (< 70%)
	// - Memory usage is reasonable (< 80%)
	if queueLen > currentWorkers*50 && cpuPercent < 70 && memoryPercent < 80 {
		if currentWorkers < wd.maxWorkers {
			targetWorkers = currentWorkers + 1
		}
	}
	
	// Scale down if:
	// - Queue is small (< 10 items per worker)
	// - CPU usage is high (> 85%)
	// - Memory usage is high (> 85%)
	if (queueLen < currentWorkers*10 || cpuPercent > 85 || memoryPercent > 85) && currentWorkers > wd.minWorkers {
		targetWorkers = currentWorkers - 1
	}
	
	// Start new workers if needed
	if targetWorkers > currentWorkers {
		for i := currentWorkers; i < targetWorkers; i++ {
			wd.wg.Add(1)
			go wd.worker(i)
		}
	}
	
	// Signal workers to exit if needed
	if targetWorkers < currentWorkers {
		exitCount := currentWorkers - targetWorkers
		for i := 0; i < exitCount; i++ {
			select {
			case wd.workerExitChan <- 1:
			default:
				// Channel full, skip
				break
			}
		}
	}
}

// dirQueueManager manages the directory queue lifecycle
func (wd *WorkDispatcher) dirQueueManager() {
	defer wd.wg.Done()
	defer close(wd.dirQueue)
	
	// Wait for context cancellation
	<-wd.ctx.Done()
}

// workQueueCloser monitors directory scanning completion and closes work queue
func (wd *WorkDispatcher) workQueueCloser() {
	defer wd.wg.Done()
	defer close(wd.workQueue)
	
	// Monitor directory workers and queue
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if directory scanning is complete
			activeDirWorkers := wd.activeDirWorkers.Load()
			dirQueueLen := len(wd.dirQueue)
			
			// If no active directory workers and directory queue is empty,
			// we're done adding new work
			if activeDirWorkers == 0 && dirQueueLen == 0 {
				// Wait a bit more to ensure all work is processed
				time.Sleep(2 * time.Second)
				
				// Final check
				if wd.activeDirWorkers.Load() == 0 && len(wd.dirQueue) == 0 {
					return
				}
			}
			
		case <-wd.ctx.Done():
			return
		}
	}
} 