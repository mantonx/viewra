// Package events provides the core event bus implementation.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// eventBus implements the EventBus interface
type eventBus struct {
	config       EventBusConfig
	logger       EventLogger
	storage      EventStorage
	metrics      EventMetrics
	
	// Internal state
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	eventChannel  chan Event
	running       bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	
	// Event buffer for in-memory storage
	recentEvents  []Event
	eventStats    EventStats
}

// NewEventBus creates a new event bus instance
func NewEventBus(config EventBusConfig, logger EventLogger, storage EventStorage, metrics EventMetrics) EventBus {
	return &eventBus{
		config:        config,
		logger:        logger,
		storage:       storage,
		metrics:       metrics,
		subscriptions: make(map[string]*Subscription),
		eventChannel:  make(chan Event, config.BufferSize),
		recentEvents:  make([]Event, 0, 100), // Keep last 100 events in memory
		stopCh:        make(chan struct{}),
	}
}

// Start starts the event bus
func (eb *eventBus) Start(ctx context.Context) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	if eb.running {
		return fmt.Errorf("event bus is already running")
	}
	
	eb.running = true
	eb.stopCh = make(chan struct{})
	
	// Start event processor
	eb.wg.Add(1)
	go eb.processEvents(ctx)
	
	// Start cleanup routine
	if eb.config.EnablePersistence && eb.config.MaxEventAge > 0 {
		eb.wg.Add(1)
		go eb.cleanupEvents(ctx)
	}
	
	eb.logger.Info("Event bus started", "buffer_size", eb.config.BufferSize)
	return nil
}

// Stop stops the event bus gracefully
func (eb *eventBus) Stop(ctx context.Context) error {
	eb.mu.Lock()
	if !eb.running {
		eb.mu.Unlock()
		return nil
	}
	eb.running = false
	eb.mu.Unlock()
	
	// Signal stop
	close(eb.stopCh)
	
	// Close event channel
	close(eb.eventChannel)
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		eb.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		eb.logger.Info("Event bus stopped gracefully")
	case <-ctx.Done():
		eb.logger.Warn("Event bus stop timed out")
		return ctx.Err()
	}
	
	// Close storage
	if eb.storage != nil {
		return eb.storage.Close()
	}
	
	return nil
}

// Publish publishes an event to the event bus
func (eb *eventBus) Publish(ctx context.Context, event Event) error {
	eb.mu.RLock()
	if !eb.running {
		eb.mu.RUnlock()
		return fmt.Errorf("event bus is not running")
	}
	eb.mu.RUnlock()
	
	// Set event ID if not provided
	if event.ID == "" {
		event.ID = eb.generateEventID()
	}
	
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Validate event
	if err := eb.validateEvent(event); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}
	
	select {
	case eb.eventChannel <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel is full, log warning and drop event
		eb.logger.Warn("Event channel full, dropping event", "event_type", event.Type, "event_id", event.ID)
		return fmt.Errorf("event channel full")
	}
}

// PublishAsync publishes an event asynchronously (non-blocking)
func (eb *eventBus) PublishAsync(event Event) error {
	eb.mu.RLock()
	if !eb.running {
		eb.mu.RUnlock()
		return fmt.Errorf("event bus is not running")
	}
	eb.mu.RUnlock()
	
	// Set event ID if not provided
	if event.ID == "" {
		event.ID = eb.generateEventID()
	}
	
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Validate event
	if err := eb.validateEvent(event); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}
	
	select {
	case eb.eventChannel <- event:
		return nil
	default:
		// Channel is full, log warning and drop event
		eb.logger.Warn("Event channel full, dropping event (async)", "event_type", event.Type, "event_id", event.ID)
		return fmt.Errorf("event channel full")
	}
}

// Subscribe subscribes to events matching the filter
func (eb *eventBus) Subscribe(ctx context.Context, filter EventFilter, handler EventHandler) (*Subscription, error) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	subscription := &Subscription{
		ID:           eb.generateSubscriptionID(),
		Filter:       filter,
		Handler:      handler,
		Subscriber:   "system", // Default subscriber
		Created:      time.Now(),
		TriggerCount: 0,
	}
	
	eb.subscriptions[subscription.ID] = subscription
	
	if eb.metrics != nil {
		eb.metrics.RecordSubscription(subscription)
	}
	
	eb.logger.Debug("New subscription created", "subscription_id", subscription.ID, "types", filter.Types)
	return subscription, nil
}

// Unsubscribe removes a subscription
func (eb *eventBus) Unsubscribe(subscriptionID string) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	if _, exists := eb.subscriptions[subscriptionID]; !exists {
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}
	
	delete(eb.subscriptions, subscriptionID)
	
	if eb.metrics != nil {
		eb.metrics.RecordUnsubscription(subscriptionID)
	}
	
	eb.logger.Debug("Subscription removed", "subscription_id", subscriptionID)
	return nil
}

// GetSubscriptions returns all active subscriptions
func (eb *eventBus) GetSubscriptions() []*Subscription {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	subscriptions := make([]*Subscription, 0, len(eb.subscriptions))
	for _, sub := range eb.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	
	return subscriptions
}

// GetEvents returns stored events based on filter and pagination
func (eb *eventBus) GetEvents(filter EventFilter, limit, offset int) ([]Event, int64, error) {
	if eb.storage != nil {
		return eb.storage.Get(context.Background(), filter, limit, offset)
	}
	
	// Fall back to in-memory events
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	filtered := FilterEvents(eb.recentEvents, filter)
	
	// Apply pagination
	total := int64(len(filtered))
	start := offset
	end := offset + limit
	
	if start >= len(filtered) {
		return []Event{}, total, nil
	}
	
	if end > len(filtered) {
		end = len(filtered)
	}
	
	return filtered[start:end], total, nil
}

// GetEventsByTimeRange returns events within a time range
func (eb *eventBus) GetEventsByTimeRange(start, end time.Time, limit, offset int) ([]Event, int64, error) {
	if eb.storage != nil {
		return eb.storage.GetByTimeRange(context.Background(), start, end, limit, offset)
	}
	
	// Fall back to in-memory events
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	var filtered []Event
	for _, event := range eb.recentEvents {
		if !event.Timestamp.Before(start) && !event.Timestamp.After(end) {
			filtered = append(filtered, event)
		}
	}
	
	// Apply pagination
	total := int64(len(filtered))
	startIdx := offset
	endIdx := offset + limit
	
	if startIdx >= len(filtered) {
		return []Event{}, total, nil
	}
	
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}
	
	return filtered[startIdx:endIdx], total, nil
}

// GetStats returns event bus statistics
func (eb *eventBus) GetStats() EventStats {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	if eb.metrics != nil {
		return eb.metrics.GetMetrics()
	}
	
	// Return basic stats
	stats := eb.eventStats
	stats.ActiveSubscriptions = len(eb.subscriptions)
	stats.RecentEvents = eb.recentEvents
	return stats
}

// DeleteEvent removes a specific event by its ID
func (eb *eventBus) DeleteEvent(ctx context.Context, eventID string) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	// Remove from recent events in memory
	for i, event := range eb.recentEvents {
		if event.ID == eventID {
			eb.recentEvents = append(eb.recentEvents[:i], eb.recentEvents[i+1:]...)
			break
		}
	}
	
	// Remove from persistent storage if available
	if eb.storage != nil {
		if err := eb.storage.DeleteByID(ctx, eventID); err != nil {
			return fmt.Errorf("failed to delete event from storage: %w", err)
		}
	}
	
	eb.logger.Debug("Event deleted", "event_id", eventID)
	return nil
}

// ClearEvents removes all events from storage
func (eb *eventBus) ClearEvents(ctx context.Context) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	// Clear in-memory events
	eb.recentEvents = make([]Event, 0, 100)
	
	// Reset event stats
	eb.eventStats = EventStats{
		EventsByType:     make(map[string]int64),
		EventsBySource:   make(map[string]int64),
		EventsByPriority: make(map[string]int64),
	}
	
	// Update metrics
	if eb.metrics != nil {
		// Reset metrics - this is implementation-dependent
		// The system will rebuild metrics as new events come in
	}
	
	// Clear persisted events if storage is available
	if eb.storage != nil {
		if err := eb.storage.DeleteAllEvents(ctx); err != nil {
			return fmt.Errorf("failed to clear persisted events: %w", err)
		}
	}
	
	eb.logger.Info("All events cleared from the system")
	return nil
}

// Health returns the health status of the event bus
func (eb *eventBus) Health() error {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	if !eb.running {
		return fmt.Errorf("event bus is not running")
	}
	
	// Check if channel is severely backed up
	channelUsage := float64(len(eb.eventChannel)) / float64(cap(eb.eventChannel))
	if channelUsage > 0.9 {
		return fmt.Errorf("event channel is %d%% full", int(channelUsage*100))
	}
	
	return nil
}

// Internal methods

// processEvents processes events from the channel
func (eb *eventBus) processEvents(ctx context.Context) {
	defer eb.wg.Done()
	
	for {
		select {
		case <-eb.stopCh:
			eb.logger.Debug("Event processor stopping")
			return
		case <-ctx.Done():
			eb.logger.Debug("Event processor stopping due to context cancellation")
			return
		case event, ok := <-eb.eventChannel:
			if !ok {
				eb.logger.Debug("Event channel closed")
				return
			}
			
			eb.handleEvent(event)
		}
	}
}

// handleEvent processes a single event
func (eb *eventBus) handleEvent(event Event) {
	eb.logger.Debug("Processing event", "type", event.Type, "id", event.ID, "source", event.Source)
	
	// Store event if persistence is enabled
	if eb.config.EnablePersistence && eb.storage != nil {
		if err := eb.storage.Store(context.Background(), event); err != nil {
			eb.logger.Error("Failed to store event", "error", err, "event_id", event.ID)
		}
	}
	
	// Add to recent events buffer
	eb.mu.Lock()
	eb.recentEvents = append(eb.recentEvents, event)
	if len(eb.recentEvents) > 100 {
		eb.recentEvents = eb.recentEvents[1:]
	}
	
	// Update stats
	eb.eventStats.TotalEvents++
	if eb.eventStats.EventsByType == nil {
		eb.eventStats.EventsByType = make(map[string]int64)
	}
	eb.eventStats.EventsByType[string(event.Type)]++
	
	if eb.eventStats.EventsBySource == nil {
		eb.eventStats.EventsBySource = make(map[string]int64)
	}
	eb.eventStats.EventsBySource[event.Source]++
	
	if eb.eventStats.EventsByPriority == nil {
		eb.eventStats.EventsByPriority = make(map[string]int64)
	}
	eb.eventStats.EventsByPriority[fmt.Sprintf("priority_%d", event.Priority)]++
	
	// Get matching subscriptions
	var matchingSubscriptions []*Subscription
	for _, sub := range eb.subscriptions {
		if MatchesFilter(event, sub.Filter) {
			matchingSubscriptions = append(matchingSubscriptions, sub)
		}
	}
	eb.mu.Unlock()
	
	// Record metrics
	if eb.metrics != nil {
		eb.metrics.RecordEvent(event)
	}
	
	// Notify subscribers
	for _, sub := range matchingSubscriptions {
		eb.notifySubscriber(sub, event)
	}
}

// notifySubscriber notifies a subscriber about an event
func (eb *eventBus) notifySubscriber(subscription *Subscription, event Event) {
	defer func() {
		if r := recover(); r != nil {
			eb.logger.Error("Panic in event handler", "subscription_id", subscription.ID, "error", r, "event_id", event.ID)
		}
	}()
	
	start := time.Now()
	
	err := subscription.Handler(event)
	if err != nil {
		eb.logger.Error("Event handler error", "subscription_id", subscription.ID, "error", err, "event_id", event.ID)
		return
	}
	
	// Update subscription stats
	eb.mu.Lock()
	subscription.TriggerCount++
	now := time.Now()
	subscription.LastTriggered = &now
	eb.mu.Unlock()
	
	duration := time.Since(start)
	eb.logger.Debug("Event handler completed", "subscription_id", subscription.ID, "duration", duration, "event_id", event.ID)
}

// cleanupEvents removes old events periodically
func (eb *eventBus) cleanupEvents(ctx context.Context) {
	defer eb.wg.Done()
	
	ticker := time.NewTicker(time.Hour) // Cleanup every hour
	defer ticker.Stop()
	
	for {
		select {
		case <-eb.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if eb.storage != nil {
				err := eb.storage.Delete(ctx, eb.config.MaxEventAge)
				if err != nil {
					eb.logger.Error("Failed to cleanup old events", "error", err)
				} else {
					eb.logger.Debug("Cleaned up old events", "max_age", eb.config.MaxEventAge)
				}
			}
		}
	}
}

// validateEvent validates an event
func (eb *eventBus) validateEvent(event Event) error {
	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}
	
	if event.Source == "" {
		return fmt.Errorf("event source is required")
	}
	
	return nil
}

// generateEventID generates a unique event ID
func (eb *eventBus) generateEventID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(bytes))
}

// generateSubscriptionID generates a unique subscription ID
func (eb *eventBus) generateSubscriptionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("sub-%s", hex.EncodeToString(bytes))
}
