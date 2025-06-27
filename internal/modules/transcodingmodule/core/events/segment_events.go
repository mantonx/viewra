// Package events provides real-time event system for streaming segments.
// This enables instant manifest updates and client notifications as segments become available.
package events

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// SegmentEventType represents different types of segment events
type SegmentEventType string

const (
	SegmentReady    SegmentEventType = "segment_ready"
	SegmentFailed   SegmentEventType = "segment_failed"
	ManifestUpdated SegmentEventType = "manifest_updated"
	StreamCompleted SegmentEventType = "stream_completed"
	StreamFailed    SegmentEventType = "stream_failed"
	ProgressUpdate  SegmentEventType = "progress_update"
	EncodingError   SegmentEventType = "encoding_error"
)

// SegmentEvent represents a streaming event
type SegmentEvent struct {
	Type        SegmentEventType       `json:"type"`
	SessionID   string                 `json:"session_id"`
	ContentHash string                 `json:"content_hash"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
}

// SegmentEventHandler processes segment events
type SegmentEventHandler func(event SegmentEvent) error

// EventBus manages real-time streaming events
type EventBus struct {
	handlers map[SegmentEventType][]SegmentEventHandler
	mu       sync.RWMutex
	logger   hclog.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewEventBus creates a new segment event bus
func NewEventBus(logger hclog.Logger) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())

	return &EventBus{
		handlers: make(map[SegmentEventType][]SegmentEventHandler),
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Subscribe adds a handler for specific event types
func (eb *EventBus) Subscribe(eventType SegmentEventType, handler SegmentEventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)

	eb.logger.Debug("Subscribed to segment event",
		"type", eventType,
		"handlers", len(eb.handlers[eventType]),
	)
}

// Publish sends an event to all registered handlers
func (eb *EventBus) Publish(event SegmentEvent) {
	eb.mu.RLock()
	handlers, exists := eb.handlers[event.Type]
	eb.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Process handlers concurrently for performance
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h SegmentEventHandler) {
			defer wg.Done()

			if err := h(event); err != nil {
				eb.logger.Error("Event handler failed",
					"type", event.Type,
					"session", event.SessionID,
					"error", err,
				)
			}
		}(handler)
	}

	// Wait for all handlers to complete (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All handlers completed
	case <-time.After(5 * time.Second):
		eb.logger.Warn("Event handlers timed out",
			"type", event.Type,
			"session", event.SessionID,
		)
	}

	eb.logger.Debug("Published segment event",
		"type", event.Type,
		"session", event.SessionID,
		"handlers", len(handlers),
	)
}

// PublishSegmentReady notifies that a new segment is available
func (eb *EventBus) PublishSegmentReady(sessionID, contentHash string, segmentIndex int, segmentPath string) {
	event := SegmentEvent{
		Type:        SegmentReady,
		SessionID:   sessionID,
		ContentHash: contentHash,
		Data: map[string]interface{}{
			"segment_index": segmentIndex,
			"segment_path":  segmentPath,
		},
	}

	eb.Publish(event)
}

// PublishManifestUpdated notifies that the manifest has been updated
func (eb *EventBus) PublishManifestUpdated(sessionID, contentHash, manifestPath string) {
	event := SegmentEvent{
		Type:        ManifestUpdated,
		SessionID:   sessionID,
		ContentHash: contentHash,
		Data: map[string]interface{}{
			"manifest_path": manifestPath,
		},
	}

	eb.Publish(event)
}

// PublishStreamCompleted notifies that streaming has finished successfully
func (eb *EventBus) PublishStreamCompleted(sessionID, contentHash string, totalSegments int, duration time.Duration) {
	event := SegmentEvent{
		Type:        StreamCompleted,
		SessionID:   sessionID,
		ContentHash: contentHash,
		Data: map[string]interface{}{
			"total_segments": totalSegments,
			"duration":       duration.String(),
		},
	}

	eb.Publish(event)
}

// PublishStreamFailed notifies that streaming has failed
func (eb *EventBus) PublishStreamFailed(sessionID, contentHash string, reason error) {
	event := SegmentEvent{
		Type:        StreamFailed,
		SessionID:   sessionID,
		ContentHash: contentHash,
		Data: map[string]interface{}{
			"error": reason.Error(),
		},
	}

	eb.Publish(event)
}

// PublishProgress notifies about encoding progress
func (eb *EventBus) PublishProgress(sessionID string, progressData map[string]interface{}) {
	event := SegmentEvent{
		Type:      ProgressUpdate,
		SessionID: sessionID,
		Data:      progressData,
	}

	eb.Publish(event)
}

// PublishError notifies about encoding errors
func (eb *EventBus) PublishError(sessionID string, errorMessage string) {
	event := SegmentEvent{
		Type:      EncodingError,
		SessionID: sessionID,
		Data: map[string]interface{}{
			"error": errorMessage,
		},
	}

	eb.Publish(event)
}

// Stop shuts down the event bus
func (eb *EventBus) Stop() {
	eb.cancel()

	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Clear all handlers
	eb.handlers = make(map[SegmentEventType][]SegmentEventHandler)

	eb.logger.Info("Segment event bus stopped")
}

// GetActiveSubscriptions returns information about active subscriptions
func (eb *EventBus) GetActiveSubscriptions() map[SegmentEventType]int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	subscriptions := make(map[SegmentEventType]int)
	for eventType, handlers := range eb.handlers {
		subscriptions[eventType] = len(handlers)
	}

	return subscriptions
}

// StreamingEventManager integrates the event bus with the streaming pipeline
type StreamingEventManager struct {
	eventBus     *EventBus
	contentStore ContentStoreInterface
	logger       hclog.Logger
}

// ContentStoreInterface defines the interface for content storage operations
type ContentStoreInterface interface {
	AddSegment(contentHash string, segmentPath string, segmentInfo interface{}) error
	Get(contentHash string) (interface{}, string, error)
}

// NewStreamingEventManager creates a new event manager
func NewStreamingEventManager(eventBus *EventBus, contentStore ContentStoreInterface, logger hclog.Logger) *StreamingEventManager {
	return &StreamingEventManager{
		eventBus:     eventBus,
		contentStore: contentStore,
		logger:       logger,
	}
}

// HandleSegmentReady processes segment ready events
func (sem *StreamingEventManager) HandleSegmentReady(event SegmentEvent) error {
	segmentIndex, ok := event.Data["segment_index"].(int)
	if !ok {
		return nil
	}

	segmentPath, ok := event.Data["segment_path"].(string)
	if !ok {
		return nil
	}

	// Add segment to content store
	segmentInfo := map[string]interface{}{
		"index": segmentIndex,
		"path":  segmentPath,
	}

	if err := sem.contentStore.AddSegment(event.ContentHash, segmentPath, segmentInfo); err != nil {
		sem.logger.Error("Failed to add segment to content store",
			"session", event.SessionID,
			"segment", segmentIndex,
			"error", err,
		)
		return err
	}

	sem.logger.Debug("Segment added to content store",
		"session", event.SessionID,
		"segment", segmentIndex,
		"hash", event.ContentHash,
	)

	return nil
}

// StartEventProcessing begins processing streaming events
func (sem *StreamingEventManager) StartEventProcessing() {
	// Subscribe to segment events
	sem.eventBus.Subscribe(SegmentReady, sem.HandleSegmentReady)

	sem.logger.Info("Streaming event processing started")
}

// StopEventProcessing stops processing streaming events
func (sem *StreamingEventManager) StopEventProcessing() {
	sem.eventBus.Stop()
	sem.logger.Info("Streaming event processing stopped")
}
