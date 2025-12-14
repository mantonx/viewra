// Package bus provides the event bus implementation for ViewRA.
// It handles pub/sub communication with non-blocking delivery,
// filtered subscriptions, and event history replay.
package bus

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

// Bus is the central event bus for ViewRA.
// It provides pub/sub communication with:
// - Non-blocking, fire-and-forget publishing
// - Filtered subscriptions
// - Ring buffer for event history replay
// - Thread-safe operation
type Bus struct {
	mu          sync.RWMutex
	subscribers map[uint64]*subscription
	nextID      atomic.Uint64
	ring        *RingBuffer[events.Event]
	logger      *slog.Logger
	closed      atomic.Bool
}

// Stats contains event bus statistics.
type Stats struct {
	SubscriberCount int
	BufferCapacity  int
	BufferCount     int
	TotalPublished  uint64
}

// New creates a new event bus with the specified ring buffer size.
// The buffer stores recent events for replay to new subscribers.
func New(bufferSize int, logger *slog.Logger) *Bus {
	return &Bus{
		subscribers: make(map[uint64]*subscription),
		ring:        NewRingBuffer[events.Event](bufferSize),
		logger:      logger,
	}
}

// Publish publishes an event to all matching subscribers.
// This is non-blocking: if a subscriber's buffer is full, the event is dropped for that subscriber.
// Events are also stored in the ring buffer for replay.
func (b *Bus) Publish(e events.Event) {
	if b.closed.Load() {
		return
	}

	// Stamp event time if not set
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// Store in ring buffer for replay
	b.ring.Add(e)

	// Fan out to subscribers (non-blocking)
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		// Check if subscriber is closed
		if sub.isClosed() {
			continue
		}

		// Apply filter if set
		if sub.filter != nil && !sub.filter(e) {
			continue
		}

		// Non-blocking send
		select {
		case sub.ch <- e:
			// Delivered
		default:
			// Subscriber buffer full, drop event
			if b.logger != nil {
				b.logger.Debug("Event dropped for slow subscriber",
					"sub_id", sub.id,
					"event_type", e.Type)
			}
		}
	}
}

// Subscribe creates a new subscription with optional filtering and replay.
// Options:
// - WithBufferSize(n): Set event buffer size (default 100)
// - WithFilter(fn): Only receive events matching filter
// - WithReplayLast(n): Replay last n events immediately
// - WithEventTypes(...): Only receive specific event types
// - WithEventPrefix("scan."): Only receive events with type prefix
func (b *Bus) Subscribe(opts ...events.SubscribeOption) events.Subscription {
	cfg := &events.SubscribeConfig{
		BufferSize: 100,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	sub := &subscription{
		id:     b.nextID.Add(1),
		ch:     make(chan events.Event, cfg.BufferSize),
		filter: cfg.Filter,
		done:   make(chan struct{}),
	}

	b.mu.Lock()
	b.subscribers[sub.id] = sub
	b.mu.Unlock()

	// Replay recent events if requested
	if cfg.ReplayLast > 0 {
		recent := b.ring.Last(cfg.ReplayLast)
		for _, e := range recent {
			// Apply filter to replayed events too
			if cfg.Filter != nil && !cfg.Filter(e) {
				continue
			}
			select {
			case sub.ch <- e:
				// Replayed
			default:
				// Skip if buffer full during replay
			}
		}
	}

	return sub
}

// Unsubscribe removes a subscription from the bus.
// The subscription's event channel will be closed.
func (b *Bus) Unsubscribe(sub events.Subscription) {
	s, ok := sub.(*subscription)
	if !ok {
		return
	}

	b.mu.Lock()
	delete(b.subscribers, s.id)
	b.mu.Unlock()

	s.Close()
}

// Close shuts down the event bus and all subscriptions.
func (b *Bus) Close() {
	if b.closed.Swap(true) {
		return // Already closed
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subscribers {
		sub.Close()
	}
	b.subscribers = make(map[uint64]*subscription)
}

// Recent returns the last n events from the ring buffer.
// Useful for diagnostics and catch-up scenarios.
func (b *Bus) Recent(n int) []events.Event {
	return b.ring.Last(n)
}

// Stats returns statistics about the event bus.
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return Stats{
		SubscriberCount: len(b.subscribers),
		BufferCapacity:  b.ring.Capacity(),
		BufferCount:     b.ring.Count(),
		TotalPublished:  b.ring.Sequence(),
	}
}
