package utils

import (
	"sync"
	"time"
)

// WorkerPool manages a pool of workers for concurrent operations
type WorkerPool struct {
	workers   int
	workQueue chan func()
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	mu        sync.RWMutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:   workers,
		workQueue: make(chan func(), workers*2), // Buffer for work items
		stopCh:    make(chan struct{}),
	}
}

// Start begins processing work items
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

// Stop stops the worker pool and waits for all workers to finish
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

// Submit adds a work item to the queue
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

// worker processes work items from the queue
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

// RateLimiter controls the rate of operations
type RateLimiter struct {
	rate     int
	interval time.Duration
	tokens   chan struct{}
	stopCh   chan struct{}
	running  bool
	mu       sync.RWMutex
}

// NewRateLimiter creates a new rate limiter
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

// Start begins token replenishment
func (rl *RateLimiter) Start() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.running {
		return
	}

	rl.running = true
	go rl.refillTokens()
}

// Stop stops token replenishment
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.running {
		return
	}

	rl.running = false
	close(rl.stopCh)
}

// Wait waits for a token to become available
func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

// TryWait attempts to get a token without blocking
func (rl *RateLimiter) TryWait() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// refillTokens replenishes tokens at the specified rate
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
