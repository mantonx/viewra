package utils

import (
	"sync"
	"time"
)

// WorkerPool manages a pool of workers for concurrent operations.
// It provides a thread-safe way to distribute work across multiple goroutines,
// useful for parallelizing CPU-intensive or I/O-bound tasks like media scanning
// and processing.
type WorkerPool struct {
	workers   int
	workQueue chan func()
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	mu        sync.RWMutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
// The work queue is buffered at 2x the worker count to allow for efficient
// work distribution without blocking submitters.
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:   workers,
		workQueue: make(chan func(), workers*2), // Buffer for work items
		stopCh:    make(chan struct{}),
	}
}

// Start begins processing work items.
// This method is idempotent - calling it multiple times has no effect
// if the pool is already running.
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		return
	}

	wp.running = true

	// Start worker goroutines
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Stop stops the worker pool and waits for all workers to finish.
// This method blocks until all workers have completed their current work
// and exited gracefully.
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.running {
		return
	}

	wp.running = false
	close(wp.stopCh)
	wp.wg.Wait()
}

// Submit adds a work item to the queue.
// Returns true if the work was successfully queued, false if the queue
// is full or the pool is not running. Non-blocking operation.
func (wp *WorkerPool) Submit(work func()) bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.running {
		return false
	}

	select {
	case wp.workQueue <- work:
		return true
	default:
		return false // Queue is full
	}
}

// worker processes work items from the queue.
// Each worker runs in its own goroutine and continues processing
// until the pool is stopped.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case work := <-wp.workQueue:
			if work != nil {
				work()
			}
		case <-wp.stopCh:
			return
		}
	}
}

// RateLimiter controls the rate of operations using a token bucket algorithm.
// This is useful for limiting API calls, file system operations, or any
// resource-constrained operations to prevent overwhelming external systems.
type RateLimiter struct {
	rate     int
	interval time.Duration
	tokens   chan struct{}
	stopCh   chan struct{}
	running  bool
	mu       sync.RWMutex
}

// NewRateLimiter creates a new rate limiter.
// Parameters:
//   - rate: number of operations allowed per interval
//   - interval: time period for the rate limit
//
// Example: NewRateLimiter(100, time.Second) allows 100 operations per second.
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		rate:     rate,
		interval: interval,
		tokens:   make(chan struct{}, rate),
		stopCh:   make(chan struct{}),
	}

	// Fill initial tokens
	for i := 0; i < rate; i++ {
		rl.tokens <- struct{}{}
	}

	return rl
}

// Start begins token replenishment.
// Tokens are replenished at a steady rate to maintain the configured
// operations per interval limit.
func (rl *RateLimiter) Start() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.running {
		return
	}

	rl.running = true
	go rl.refillTokens()
}

// Stop stops token replenishment.
// After stopping, no new tokens will be added to the bucket.
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.running {
		return
	}

	rl.running = false
	close(rl.stopCh)
}

// Wait waits for a token to become available.
// This method blocks until a token can be consumed, ensuring
// the rate limit is respected.
func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

// TryWait attempts to get a token without blocking.
// Returns true if a token was available and consumed, false otherwise.
// Useful for non-blocking rate limiting where operations can be skipped.
func (rl *RateLimiter) TryWait() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// refillTokens replenishes tokens at the specified rate.
// Runs in a separate goroutine and adds tokens to the bucket at
// regular intervals to maintain the configured rate limit.
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(rl.interval / time.Duration(rl.rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Try to add a token if there's space
			select {
			case rl.tokens <- struct{}{}:
			default:
				// Token bucket is full, skip
			}
		case <-rl.stopCh:
			return
		}
	}
}
