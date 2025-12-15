package processing

import (
	"runtime/debug"
	"sync"
)

// WorkerPool manages a pool of goroutines that process items from an input channel.
// It supports two patterns:
//   - Fan-out: Workers process items and discard results (Output is nil)
//   - Pipeline: Workers process items and send results to an output channel
//
// Features:
//   - Per-item panic recovery (one bad item doesn't kill the worker)
//   - Graceful shutdown when input channel closes
//   - Automatic output channel closure after all workers complete
//
// Usage (fan-out pattern):
//
//	pool := &WorkerPool[*Checkpoint]{
//	    NumWorkers: 4,
//	    Input:      checkpoints,
//	    Process:    func(id int, cp *Checkpoint) { processCheckpoint(cp) },
//	}
//	pool.Run()
//
// Usage (pipeline pattern):
//
//	output := make(chan *Result, 100)
//	pool := &WorkerPool[*Job]{
//	    NumWorkers: 8,
//	    Input:      jobs,
//	    Output:     output,
//	    Transform:  func(id int, job *Job) *Result { return hash(job) },
//	    OnPanic:    func(id int, job *Job, r any) *Result { return errorResult(job) },
//	}
//	pool.Run() // Closes output channel when done
type WorkerPool[In any, Out any] struct {
	// NumWorkers is the number of concurrent worker goroutines.
	NumWorkers int

	// Input is the channel workers read from. Workers exit when this closes.
	Input <-chan In

	// Output is an optional channel for pipeline pattern. If set, Transform results
	// are sent here. The channel is closed automatically when all workers complete.
	// For fan-out pattern (no output needed), leave this nil.
	Output chan<- Out

	// Process handles each input item (fan-out pattern).
	// Called when Output is nil. Use this for side-effect-only processing.
	// The workerID (0 to NumWorkers-1) is provided for logging/debugging.
	Process func(workerID int, item In)

	// Transform converts input to output (pipeline pattern).
	// Called when Output is set. The returned value is sent to Output.
	// The workerID (0 to NumWorkers-1) is provided for logging/debugging.
	Transform func(workerID int, item In) Out

	// OnPanic is called when processing an item panics.
	// For fan-out pattern: just log/record the panic.
	// For pipeline pattern: return a fallback value to send to Output.
	// If nil, panics are recovered but silently ignored.
	OnPanic func(workerID int, item In, recovered any) Out
}

// Run starts all workers and blocks until all complete.
// For pipeline pattern, it closes the Output channel after all workers finish.
func (p *WorkerPool[In, Out]) Run() {
	var wg sync.WaitGroup

	for w := 0; w < p.NumWorkers; w++ {
		wg.Add(1)
		go p.worker(w, &wg)
	}

	wg.Wait()

	// Close output channel after all workers complete (pipeline pattern)
	if p.Output != nil {
		close(p.Output)
	}
}

// worker processes items from Input until the channel closes.
func (p *WorkerPool[In, Out]) worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	for item := range p.Input {
		p.processItem(id, item)
	}
}

// processItem handles a single item with panic recovery.
func (p *WorkerPool[In, Out]) processItem(workerID int, item In) {
	defer func() {
		if r := recover(); r != nil {
			p.handlePanic(workerID, item, r)
		}
	}()

	if p.Output != nil && p.Transform != nil {
		// Pipeline pattern: transform and send to output
		result := p.Transform(workerID, item)
		p.Output <- result
	} else if p.Process != nil {
		// Fan-out pattern: process with side effects only
		p.Process(workerID, item)
	}
}

// handlePanic processes a recovered panic.
func (p *WorkerPool[In, Out]) handlePanic(workerID int, item In, recovered any) {
	if p.OnPanic == nil {
		return
	}

	result := p.OnPanic(workerID, item, recovered)

	// For pipeline pattern, send the fallback result to output
	if p.Output != nil {
		p.Output <- result
	}
}

// RunWithInit starts workers with per-worker initialization.
// The initFn is called once per worker before processing begins.
// This is useful when workers need per-goroutine state (e.g., a hasher instance).
func (p *WorkerPool[In, Out]) RunWithInit(initFn func(workerID int)) {
	var wg sync.WaitGroup

	for w := 0; w < p.NumWorkers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if initFn != nil {
				initFn(id)
			}
			for item := range p.Input {
				p.processItem(id, item)
			}
		}(w)
	}

	wg.Wait()

	if p.Output != nil {
		close(p.Output)
	}
}

// PanicInfo contains details about a recovered panic for logging.
type PanicInfo struct {
	WorkerID  int
	Recovered any
	Stack     string
}

// NewPanicInfo creates a PanicInfo with the current stack trace.
func NewPanicInfo(workerID int, recovered any) PanicInfo {
	return PanicInfo{
		WorkerID:  workerID,
		Recovered: recovered,
		Stack:     string(debug.Stack()),
	}
}
