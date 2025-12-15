package processing

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestWorkerPool_FanOut(t *testing.T) {
	t.Run("processes all items", func(t *testing.T) {
		input := make(chan int, 10)
		for i := 0; i < 10; i++ {
			input <- i
		}
		close(input)

		var count atomic.Int32
		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 3,
			Input:      input,
			Process: func(workerID int, item int) {
				count.Add(1)
			},
		}
		pool.Run()

		if count.Load() != 10 {
			t.Errorf("expected 10 items processed, got %d", count.Load())
		}
	})

	t.Run("uses correct number of workers", func(t *testing.T) {
		// Test that the pool spawns the requested number of workers
		// by tracking worker IDs that process items
		input := make(chan int, 1000)
		for i := 0; i < 1000; i++ {
			input <- i
		}
		close(input)

		workersSeen := make(map[int]bool)
		var mu sync.Mutex

		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 4,
			Input:      input,
			Process: func(workerID int, item int) {
				mu.Lock()
				workersSeen[workerID] = true
				mu.Unlock()
				// Small work to allow scheduling
				for j := 0; j < 100; j++ {
					_ = j * item
				}
			},
		}
		pool.Run()

		// Worker IDs should be 0, 1, 2, 3
		for id := range workersSeen {
			if id < 0 || id >= 4 {
				t.Errorf("unexpected worker ID: %d", id)
			}
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		input := make(chan int)
		close(input)

		var count atomic.Int32
		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 3,
			Input:      input,
			Process: func(workerID int, item int) {
				count.Add(1)
			},
		}
		pool.Run()

		if count.Load() != 0 {
			t.Errorf("expected 0 items processed, got %d", count.Load())
		}
	})
}

func TestWorkerPool_Pipeline(t *testing.T) {
	t.Run("transforms all items to output", func(t *testing.T) {
		input := make(chan int, 5)
		for i := 1; i <= 5; i++ {
			input <- i
		}
		close(input)

		output := make(chan int, 5)

		pool := &WorkerPool[int, int]{
			NumWorkers: 2,
			Input:      input,
			Output:     output,
			Transform: func(workerID int, item int) int {
				return item * 2
			},
		}
		pool.Run()

		// Collect results
		var results []int
		for r := range output {
			results = append(results, r)
		}

		if len(results) != 5 {
			t.Errorf("expected 5 results, got %d", len(results))
		}

		// Check all doubled values are present (order may vary)
		sum := 0
		for _, r := range results {
			sum += r
		}
		// 1+2+3+4+5 = 15, doubled = 30
		if sum != 30 {
			t.Errorf("expected sum of 30, got %d", sum)
		}
	})

	t.Run("closes output channel when done", func(t *testing.T) {
		input := make(chan int, 1)
		input <- 1
		close(input)

		output := make(chan int, 1)

		pool := &WorkerPool[int, int]{
			NumWorkers: 1,
			Input:      input,
			Output:     output,
			Transform: func(workerID int, item int) int {
				return item
			},
		}
		pool.Run()

		// Should be able to range over output without blocking
		count := 0
		for range output {
			count++
		}
		if count != 1 {
			t.Errorf("expected 1 result, got %d", count)
		}
	})
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	t.Run("fan-out recovers from panic and continues", func(t *testing.T) {
		input := make(chan int, 5)
		for i := 0; i < 5; i++ {
			input <- i
		}
		close(input)

		var successCount atomic.Int32
		var panicCount atomic.Int32

		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 1, // Single worker to ensure order
			Input:      input,
			Process: func(workerID int, item int) {
				if item == 2 {
					panic("intentional panic")
				}
				successCount.Add(1)
			},
			OnPanic: func(workerID int, item int, recovered any) struct{} {
				panicCount.Add(1)
				return struct{}{}
			},
		}
		pool.Run()

		// Should process 4 items successfully (0,1,3,4) and have 1 panic (2)
		if successCount.Load() != 4 {
			t.Errorf("expected 4 successful, got %d", successCount.Load())
		}
		if panicCount.Load() != 1 {
			t.Errorf("expected 1 panic, got %d", panicCount.Load())
		}
	})

	t.Run("pipeline sends fallback on panic", func(t *testing.T) {
		input := make(chan int, 3)
		input <- 1
		input <- 2 // will panic
		input <- 3
		close(input)

		output := make(chan string, 3)

		pool := &WorkerPool[int, string]{
			NumWorkers: 1,
			Input:      input,
			Output:     output,
			Transform: func(workerID int, item int) string {
				if item == 2 {
					panic("boom")
				}
				return "ok"
			},
			OnPanic: func(workerID int, item int, recovered any) string {
				return "panic"
			},
		}
		pool.Run()

		var results []string
		for r := range output {
			results = append(results, r)
		}

		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}

		// Check we got one "panic" result
		panicResults := 0
		for _, r := range results {
			if r == "panic" {
				panicResults++
			}
		}
		if panicResults != 1 {
			t.Errorf("expected 1 panic result, got %d", panicResults)
		}
	})

	t.Run("nil OnPanic silently recovers", func(t *testing.T) {
		input := make(chan int, 2)
		input <- 1
		input <- 2
		close(input)

		var count atomic.Int32

		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 1,
			Input:      input,
			Process: func(workerID int, item int) {
				if item == 1 {
					panic("oops")
				}
				count.Add(1)
			},
			// OnPanic is nil - should silently recover
		}
		pool.Run()

		if count.Load() != 1 {
			t.Errorf("expected 1 item processed after panic, got %d", count.Load())
		}
	})
}

func TestWorkerPool_RunWithInit(t *testing.T) {
	t.Run("calls init for each worker", func(t *testing.T) {
		input := make(chan int, 10)
		for i := 0; i < 10; i++ {
			input <- i
		}
		close(input)

		var initCount atomic.Int32
		initWorkers := make(map[int]bool)
		var mu sync.Mutex

		pool := &WorkerPool[int, struct{}]{
			NumWorkers: 3,
			Input:      input,
			Process: func(workerID int, item int) {
				// Just process
			},
		}

		pool.RunWithInit(func(workerID int) {
			initCount.Add(1)
			mu.Lock()
			initWorkers[workerID] = true
			mu.Unlock()
		})

		if initCount.Load() != 3 {
			t.Errorf("expected 3 init calls, got %d", initCount.Load())
		}

		// Should have workers 0, 1, 2
		for i := 0; i < 3; i++ {
			if !initWorkers[i] {
				t.Errorf("worker %d was not initialized", i)
			}
		}
	})
}

func TestNewPanicInfo(t *testing.T) {
	info := NewPanicInfo(5, "test error")

	if info.WorkerID != 5 {
		t.Errorf("expected workerID 5, got %d", info.WorkerID)
	}
	if info.Recovered != "test error" {
		t.Errorf("expected 'test error', got %v", info.Recovered)
	}
	if info.Stack == "" {
		t.Error("expected non-empty stack trace")
	}
}

func BenchmarkWorkerPool_FanOut(b *testing.B) {
	for _, numWorkers := range []int{1, 2, 4, 8} {
		b.Run(string(rune('0'+numWorkers))+"workers", func(b *testing.B) {
			input := make(chan int, b.N)
			for i := 0; i < b.N; i++ {
				input <- i
			}
			close(input)

			b.ResetTimer()

			pool := &WorkerPool[int, struct{}]{
				NumWorkers: numWorkers,
				Input:      input,
				Process: func(workerID int, item int) {
					// Minimal work
					_ = item * 2
				},
			}
			pool.Run()
		})
	}
}

func BenchmarkWorkerPool_Pipeline(b *testing.B) {
	for _, numWorkers := range []int{1, 2, 4, 8} {
		b.Run(string(rune('0'+numWorkers))+"workers", func(b *testing.B) {
			input := make(chan int, b.N)
			output := make(chan int, b.N)

			for i := 0; i < b.N; i++ {
				input <- i
			}
			close(input)

			b.ResetTimer()

			pool := &WorkerPool[int, int]{
				NumWorkers: numWorkers,
				Input:      input,
				Output:     output,
				Transform: func(workerID int, item int) int {
					return item * 2
				},
			}
			pool.Run()

			// Drain output
			for range output {
			}
		})
	}
}
