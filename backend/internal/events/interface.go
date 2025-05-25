// Package events provides the core event bus interface and implementation.
package events

import (
	"context"
	"time"
)

// EventBus defines the interface for the event bus system
type EventBus interface {
	// Publish publishes an event to the event bus
	Publish(ctx context.Context, event Event) error
	
	// PublishAsync publishes an event asynchronously (non-blocking)
	PublishAsync(event Event) error
	
	// Subscribe subscribes to events matching the filter
	Subscribe(ctx context.Context, filter EventFilter, handler EventHandler) (*Subscription, error)
	
	// Unsubscribe removes a subscription
	Unsubscribe(subscriptionID string) error
	
	// GetSubscriptions returns all active subscriptions
	GetSubscriptions() []*Subscription
	
	// GetEvents returns stored events based on filter and pagination
	GetEvents(filter EventFilter, limit, offset int) ([]Event, int64, error)
	
	// GetEventsByTimeRange returns events within a time range
	GetEventsByTimeRange(start, end time.Time, limit, offset int) ([]Event, int64, error)
	
	// GetStats returns event bus statistics
	GetStats() EventStats
	
	// ClearEvents removes all events from storage
	ClearEvents(ctx context.Context) error
	
	// Start starts the event bus
	Start(ctx context.Context) error
	
	// Stop stops the event bus gracefully
	Stop(ctx context.Context) error
	
	// Health returns the health status of the event bus
	Health() error
}

// EventLogger defines the logging interface for events
type EventLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// EventStorage defines the interface for persisting events
type EventStorage interface {
	// Store stores an event
	Store(ctx context.Context, event Event) error
	
	// Get retrieves events based on filter
	Get(ctx context.Context, filter EventFilter, limit, offset int) ([]Event, int64, error)
	
	// GetByTimeRange retrieves events within a time range
	GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]Event, int64, error)
	
	// Delete removes events older than the specified duration
	Delete(ctx context.Context, olderThan time.Duration) error
	
	// DeleteAllEvents removes all events from storage
	DeleteAllEvents(ctx context.Context) error
	
	// Count returns the total number of stored events
	Count(ctx context.Context) (int64, error)
	
	// Close closes the storage
	Close() error
}

// EventMetrics defines the interface for event metrics collection
type EventMetrics interface {
	// RecordEvent records an event for metrics
	RecordEvent(event Event)
	
	// RecordSubscription records a subscription event
	RecordSubscription(subscription *Subscription)
	
	// RecordUnsubscription records an unsubscription event
	RecordUnsubscription(subscriptionID string)
	
	// GetMetrics returns current metrics
	GetMetrics() EventStats
}

// EventBusFactory creates event bus instances
type EventBusFactory interface {
	// CreateEventBus creates a new event bus instance
	CreateEventBus(config EventBusConfig, logger EventLogger, storage EventStorage) EventBus
}

// SystemEventBus represents the system-wide event bus
// This will be implemented in bus.go
type SystemEventBus struct {
	config      EventBusConfig
	logger      EventLogger
	storage     EventStorage
	metrics     EventMetrics
	
	// Internal state (will be defined in implementation)
}

// Helper functions for creating events

// NewEvent creates a new event with default values
func NewEvent(eventType EventType, source string, title string, message string) Event {
	return Event{
		ID:        generateEventID(),
		Type:      eventType,
		Source:    source,
		Title:     title,
		Message:   message,
		Data:      make(map[string]interface{}),
		Priority:  PriorityNormal,
		Tags:      []string{},
		Timestamp: time.Now(),
	}
}

// NewEventWithData creates a new event with structured data
func NewEventWithData(eventType EventType, source string, title string, message string, data map[string]interface{}) Event {
	event := NewEvent(eventType, source, title, message)
	event.Data = data
	return event
}

// NewSystemEvent creates a system event
func NewSystemEvent(eventType EventType, title string, message string) Event {
	return NewEvent(eventType, "system", title, message)
}

// NewPluginEvent creates a plugin event
func NewPluginEvent(eventType EventType, pluginID string, title string, message string) Event {
	return NewEvent(eventType, "plugin:"+pluginID, title, message)
}

// NewUserEvent creates a user event
func NewUserEvent(eventType EventType, userID string, title string, message string) Event {
	return NewEvent(eventType, "user:"+userID, title, message)
}

// Event builder pattern helpers

// EventBuilder provides a fluent interface for building events
type EventBuilder struct {
	event Event
}

// NewEventBuilder creates a new event builder
func NewEventBuilder(eventType EventType, source string) *EventBuilder {
	return &EventBuilder{
		event: NewEvent(eventType, source, "", ""),
	}
}

// WithTitle sets the event title
func (b *EventBuilder) WithTitle(title string) *EventBuilder {
	b.event.Title = title
	return b
}

// WithMessage sets the event message
func (b *EventBuilder) WithMessage(message string) *EventBuilder {
	b.event.Message = message
	return b
}

// WithData sets the event data
func (b *EventBuilder) WithData(data map[string]interface{}) *EventBuilder {
	b.event.Data = data
	return b
}

// WithDataValue adds a single data value
func (b *EventBuilder) WithDataValue(key string, value interface{}) *EventBuilder {
	if b.event.Data == nil {
		b.event.Data = make(map[string]interface{})
	}
	b.event.Data[key] = value
	return b
}

// WithPriority sets the event priority
func (b *EventBuilder) WithPriority(priority EventPriority) *EventBuilder {
	b.event.Priority = priority
	return b
}

// WithTags sets the event tags
func (b *EventBuilder) WithTags(tags ...string) *EventBuilder {
	b.event.Tags = tags
	return b
}

// WithTag adds a single tag
func (b *EventBuilder) WithTag(tag string) *EventBuilder {
	b.event.Tags = append(b.event.Tags, tag)
	return b
}

// WithTarget sets the event target
func (b *EventBuilder) WithTarget(target string) *EventBuilder {
	b.event.Target = target
	return b
}

// WithTTL sets the event time-to-live
func (b *EventBuilder) WithTTL(ttl time.Duration) *EventBuilder {
	b.event.TTL = &ttl
	return b
}

// Build returns the constructed event
func (b *EventBuilder) Build() Event {
	if b.event.ID == "" {
		b.event.ID = generateEventID()
	}
	if b.event.Timestamp.IsZero() {
		b.event.Timestamp = time.Now()
	}
	return b.event
}

// Utility functions

// generateEventID generates a unique event ID
func generateEventID() string {
	// Simple timestamp-based ID for now
	// In production, you might want to use UUID or similar
	return time.Now().Format("20060102150405.000000")
}

// MatchesFilter checks if an event matches the given filter
func MatchesFilter(event Event, filter EventFilter) bool {
	// Check event types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check sources
	if len(filter.Sources) > 0 {
		found := false
		for _, s := range filter.Sources {
			if event.Source == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check tags
	if len(filter.Tags) > 0 {
		found := false
		for _, filterTag := range filter.Tags {
			for _, eventTag := range event.Tags {
				if eventTag == filterTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check priority
	if filter.Priority != nil && event.Priority < *filter.Priority {
		return false
	}
	
	return true
}

// FilterEvents filters a slice of events based on the filter
func FilterEvents(events []Event, filter EventFilter) []Event {
	var filtered []Event
	for _, event := range events {
		if MatchesFilter(event, filter) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
