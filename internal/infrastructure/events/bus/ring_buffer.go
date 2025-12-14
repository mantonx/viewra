package bus

import (
	"sync"
)

// RingBuffer is a thread-safe circular buffer for storing recent events.
// When full, new items overwrite the oldest items.
type RingBuffer[T any] struct {
	mu       sync.RWMutex
	items    []T
	size     int
	head     int    // Next write position
	count    int    // Current number of items
	sequence uint64 // Monotonic sequence number
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}
	return &RingBuffer[T]{
		items: make([]T, capacity),
		size:  capacity,
	}
}

// Add adds an item to the buffer. If full, overwrites the oldest item.
func (rb *RingBuffer[T]) Add(item T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.head] = item
	rb.head = (rb.head + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	}
	rb.sequence++
}

// Last returns the last n items in chronological order (oldest to newest).
// If fewer than n items exist, returns all available items.
func (rb *RingBuffer[T]) Last(n int) []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 {
		return nil
	}

	if n > rb.count {
		n = rb.count
	}

	result := make([]T, n)

	// Calculate start position (oldest item we want)
	start := (rb.head - n + rb.size) % rb.size

	for i := 0; i < n; i++ {
		idx := (start + i) % rb.size
		result[i] = rb.items[idx]
	}

	return result
}

// Count returns the current number of items in the buffer.
func (rb *RingBuffer[T]) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Sequence returns the total number of items ever added.
// Useful for detecting missed events.
func (rb *RingBuffer[T]) Sequence() uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.sequence
}

// Clear empties the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Zero out the items for GC
	var zero T
	for i := range rb.items {
		rb.items[i] = zero
	}

	rb.head = 0
	rb.count = 0
}

// Capacity returns the maximum number of items the buffer can hold.
func (rb *RingBuffer[T]) Capacity() int {
	return rb.size
}
