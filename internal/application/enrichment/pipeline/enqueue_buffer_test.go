package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// mockEnqueueManager tracks enqueue calls for testing.
type mockEnqueueManager struct {
	mu               sync.Mutex
	singleCalls      int
	batchCalls       int
	batchTotalItems  int
	singleErr        error
	batchErr         error
	enqueuedItems    []EnqueueItem
}

func (m *mockEnqueueManager) EnqueueFirstStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.singleCalls++
	if m.singleErr != nil {
		return m.singleErr
	}
	m.enqueuedItems = append(m.enqueuedItems, EnqueueItem{
		MediaID:   mediaID,
		LibraryID: libraryID,
		MediaType: mediaType,
	})
	return nil
}

func (m *mockEnqueueManager) EnqueueFirstStageBatch(ctx context.Context, items []EnqueueItem) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchCalls++
	m.batchTotalItems += len(items)
	if m.batchErr != nil {
		return 0, m.batchErr
	}
	m.enqueuedItems = append(m.enqueuedItems, items...)
	return len(items), nil
}

func (m *mockEnqueueManager) getSingleCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.singleCalls
}

func (m *mockEnqueueManager) getBatchCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchCalls
}

func (m *mockEnqueueManager) getBatchTotalItems() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchTotalItems
}

func (m *mockEnqueueManager) getEnqueuedItems() []EnqueueItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]EnqueueItem{}, m.enqueuedItems...)
}

func TestEnqueueBuffer_BatchFlush(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(5),
		WithFlushInterval(100*time.Millisecond))

	ctx := context.Background()
	buffer.Start(ctx)

	// Enqueue 5 items (should trigger batch flush)
	for i := 0; i < 5; i++ {
		buffer.Enqueue(int64(i+1), 1, enrichment.MediaTypeMovie, 0)
	}

	// Wait for flush
	time.Sleep(50 * time.Millisecond)

	// Check that batch was called
	if manager.getBatchCalls() != 1 {
		t.Errorf("expected 1 batch call, got %d", manager.getBatchCalls())
	}
	if manager.getBatchTotalItems() != 5 {
		t.Errorf("expected 5 items in batch, got %d", manager.getBatchTotalItems())
	}

	buffer.Stop()
}

func TestEnqueueBuffer_TimeFlush(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(100), // Large batch size
		WithFlushInterval(50*time.Millisecond)) // Short interval

	ctx := context.Background()
	buffer.Start(ctx)

	// Enqueue just 3 items (won't hit batch size)
	for i := 0; i < 3; i++ {
		buffer.Enqueue(int64(i+1), 1, enrichment.MediaTypeMovie, 0)
	}

	// Wait for time-based flush
	time.Sleep(100 * time.Millisecond)

	// Check that batch was called due to time flush
	if manager.getBatchCalls() != 1 {
		t.Errorf("expected 1 batch call (time flush), got %d", manager.getBatchCalls())
	}
	if manager.getBatchTotalItems() != 3 {
		t.Errorf("expected 3 items in batch, got %d", manager.getBatchTotalItems())
	}

	buffer.Stop()
}

func TestEnqueueBuffer_StopFlushesRemaining(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(100),
		WithFlushInterval(10*time.Second)) // Long interval

	ctx := context.Background()
	buffer.Start(ctx)

	// Enqueue items
	for i := 0; i < 7; i++ {
		buffer.Enqueue(int64(i+1), 1, enrichment.MediaTypeMovie, 0)
	}

	// Stop should flush remaining items
	buffer.Stop()

	if manager.getBatchTotalItems() != 7 {
		t.Errorf("expected 7 items flushed on stop, got %d", manager.getBatchTotalItems())
	}
}

func TestEnqueueBuffer_MultipleBatches(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(3),
		WithFlushInterval(1*time.Second))

	ctx := context.Background()
	buffer.Start(ctx)

	// Enqueue 10 items (should trigger 3 batch flushes + 1 final)
	for i := 0; i < 10; i++ {
		buffer.Enqueue(int64(i+1), 1, enrichment.MediaTypeMovie, 0)
		time.Sleep(5 * time.Millisecond) // Small delay to allow processing
	}

	buffer.Stop()

	// Should have 4 batch calls (3+3+3+1)
	if manager.getBatchTotalItems() != 10 {
		t.Errorf("expected 10 total items, got %d", manager.getBatchTotalItems())
	}
}

func TestEnqueueBuffer_Pending(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(100),
		WithFlushInterval(10*time.Second))

	// Don't start - just check pending count
	buffer.Enqueue(1, 1, enrichment.MediaTypeMovie, 0)
	buffer.Enqueue(2, 1, enrichment.MediaTypeMovie, 0)

	if buffer.Pending() != 2 {
		t.Errorf("expected 2 pending, got %d", buffer.Pending())
	}
}

func TestEnqueueBuffer_Priority(t *testing.T) {
	manager := &mockEnqueueManager{}
	buffer := NewEnqueueBuffer(manager, testLogger(),
		WithBatchSize(2),
		WithFlushInterval(100*time.Millisecond))

	ctx := context.Background()
	buffer.Start(ctx)

	// Enqueue with different priorities
	buffer.Enqueue(1, 1, enrichment.MediaTypeMovie, 10)
	buffer.Enqueue(2, 1, enrichment.MediaTypeMovie, 5)

	// Wait for flush
	time.Sleep(50 * time.Millisecond)
	buffer.Stop()

	items := manager.getEnqueuedItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Check priorities are preserved
	found10 := false
	found5 := false
	for _, item := range items {
		if item.Priority == 10 {
			found10 = true
		}
		if item.Priority == 5 {
			found5 = true
		}
	}
	if !found10 || !found5 {
		t.Error("priorities not preserved in batch")
	}
}
