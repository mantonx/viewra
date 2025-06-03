// Package events provides database storage implementation for events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// SystemEvent represents a system event in the database
type SystemEvent struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	EventID   string    `gorm:"uniqueIndex;not null" json:"event_id"`
	Type      string    `gorm:"not null;index" json:"type"`
	Source    string    `gorm:"not null;index" json:"source"`
	Target    string    `gorm:"index" json:"target"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Data      string    `gorm:"type:text" json:"data"` // JSON-encoded event data
	Priority  int       `gorm:"not null;index" json:"priority"`
	Tags      string    `gorm:"type:text" json:"tags"` // JSON-encoded tags
	TTL       *int64    `json:"ttl"`                   // TTL in seconds
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for SystemEvent
func (SystemEvent) TableName() string {
	return "system_events"
}

// ToEvent converts a SystemEvent to an Event
func (se *SystemEvent) ToEvent() (Event, error) {
	event := Event{
		ID:        se.EventID,
		Type:      EventType(se.Type),
		Source:    se.Source,
		Target:    se.Target,
		Title:     se.Title,
		Message:   se.Message,
		Priority:  EventPriority(se.Priority),
		Timestamp: se.CreatedAt,
	}

	// Parse data
	if se.Data != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(se.Data), &data); err != nil {
			return event, fmt.Errorf("failed to unmarshal event data: %w", err)
		}
		event.Data = data
	} else {
		event.Data = make(map[string]interface{})
	}

	// Parse tags
	if se.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(se.Tags), &tags); err != nil {
			return event, fmt.Errorf("failed to unmarshal event tags: %w", err)
		}
		event.Tags = tags
	} else {
		event.Tags = []string{}
	}

	// Parse TTL
	if se.TTL != nil {
		ttl := time.Duration(*se.TTL) * time.Second
		event.TTL = &ttl
	}

	return event, nil
}

// FromEvent creates a SystemEvent from an Event
func (se *SystemEvent) FromEvent(event Event) error {
	se.EventID = event.ID
	se.Type = string(event.Type)
	se.Source = event.Source
	se.Target = event.Target
	se.Title = event.Title
	se.Message = event.Message
	se.Priority = int(event.Priority)
	se.CreatedAt = event.Timestamp
	se.UpdatedAt = time.Now()

	// Serialize data
	if event.Data != nil {
		dataBytes, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		se.Data = string(dataBytes)
	}

	// Serialize tags
	if event.Tags != nil {
		tagsBytes, err := json.Marshal(event.Tags)
		if err != nil {
			return fmt.Errorf("failed to marshal event tags: %w", err)
		}
		se.Tags = string(tagsBytes)
	}

	// Serialize TTL
	if event.TTL != nil {
		ttlSeconds := int64(event.TTL.Seconds())
		se.TTL = &ttlSeconds
	}

	return nil
}

// databaseEventStorage implements EventStorage using GORM
type databaseEventStorage struct {
	db *gorm.DB
}

// NewDatabaseEventStorage creates a new database event storage
func NewDatabaseEventStorage(db *gorm.DB) EventStorage {
	return &databaseEventStorage{db: db}
}

// Store stores an event in the database
func (s *databaseEventStorage) Store(ctx context.Context, event Event) error {
	var systemEvent SystemEvent
	if err := systemEvent.FromEvent(event); err != nil {
		return fmt.Errorf("failed to convert event: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(&systemEvent).Error; err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	return nil
}

// Get retrieves events based on filter
func (s *databaseEventStorage) Get(ctx context.Context, filter EventFilter, limit, offset int) ([]Event, int64, error) {
	query := s.db.WithContext(ctx).Model(&SystemEvent{})

	// Apply filters
	if len(filter.Types) > 0 {
		types := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			types[i] = string(t)
		}
		query = query.Where("type IN ?", types)
	}

	if len(filter.Sources) > 0 {
		query = query.Where("source IN ?", filter.Sources)
	}

	if filter.Priority != nil {
		query = query.Where("priority >= ?", int(*filter.Priority))
	}

	// Tag filtering is more complex and might require a different approach
	// For now, we'll skip tag filtering in the database query

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	// Get events with pagination
	var systemEvents []SystemEvent
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&systemEvents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve events: %w", err)
	}

	// Convert to events
	events := make([]Event, 0, len(systemEvents))
	for _, se := range systemEvents {
		event, err := se.ToEvent()
		if err != nil {
			continue // Skip invalid events
		}

		// Apply tag filter if needed
		if len(filter.Tags) > 0 {
			if !MatchesFilter(event, filter) {
				continue
			}
		}

		events = append(events, event)
	}

	return events, total, nil
}

// GetByTimeRange retrieves events within a time range
func (s *databaseEventStorage) GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]Event, int64, error) {
	query := s.db.WithContext(ctx).Model(&SystemEvent{}).
		Where("created_at >= ? AND created_at <= ?", start, end)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	// Get events with pagination
	var systemEvents []SystemEvent
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&systemEvents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve events: %w", err)
	}

	// Convert to events
	events := make([]Event, 0, len(systemEvents))
	for _, se := range systemEvents {
		event, err := se.ToEvent()
		if err != nil {
			continue // Skip invalid events
		}
		events = append(events, event)
	}

	return events, total, nil
}

// Delete removes events older than the specified duration
func (s *databaseEventStorage) Delete(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	result := s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&SystemEvent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete old events: %w", result.Error)
	}

	return nil
}

// DeleteByID removes a specific event by its ID
func (s *databaseEventStorage) DeleteByID(ctx context.Context, eventID string) error {
	result := s.db.WithContext(ctx).Where("event_id = ?", eventID).Delete(&SystemEvent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete event %s: %w", eventID, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}

	return nil
}

// Count returns the total number of stored events
func (s *databaseEventStorage) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&SystemEvent{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}
	return count, nil
}

// DeleteAllEvents removes all events from the database
func (s *databaseEventStorage) DeleteAllEvents(ctx context.Context) error {
	result := s.db.WithContext(ctx).Exec("DELETE FROM system_events")
	if result.Error != nil {
		return fmt.Errorf("failed to delete all events: %w", result.Error)
	}

	return nil
}

// Close closes the storage (no-op for database storage)
func (s *databaseEventStorage) Close() error {
	return nil
}

// memoryEventStorage implements EventStorage in memory (for testing or small deployments)
type memoryEventStorage struct {
	events []Event
	mutex  sync.RWMutex
}

// NewMemoryEventStorage creates a new in-memory event storage
func NewMemoryEventStorage() EventStorage {
	return &memoryEventStorage{
		events: make([]Event, 0),
	}
}

// Store stores an event in memory
func (s *memoryEventStorage) Store(ctx context.Context, event Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.events = append(s.events, event)
	return nil
}

// Get retrieves events based on filter
func (s *memoryEventStorage) Get(ctx context.Context, filter EventFilter, limit, offset int) ([]Event, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	filtered := FilterEvents(s.events, filter)
	total := int64(len(filtered))

	// Sort by timestamp descending
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].Timestamp.Before(filtered[j].Timestamp) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	// Apply pagination
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

// GetByTimeRange retrieves events within a time range
func (s *memoryEventStorage) GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]Event, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var filtered []Event
	for _, event := range s.events {
		if !event.Timestamp.Before(start) && !event.Timestamp.After(end) {
			filtered = append(filtered, event)
		}
	}

	total := int64(len(filtered))

	// Sort by timestamp descending
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].Timestamp.Before(filtered[j].Timestamp) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	// Apply pagination
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

// Delete removes events older than the specified duration
func (s *memoryEventStorage) Delete(ctx context.Context, olderThan time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-olderThan)

	var filtered []Event
	for _, event := range s.events {
		if event.Timestamp.After(cutoff) {
			filtered = append(filtered, event)
		}
	}

	s.events = filtered
	return nil
}

// DeleteByID removes a specific event by its ID
func (s *memoryEventStorage) DeleteByID(ctx context.Context, eventID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, event := range s.events {
		if event.ID == eventID {
			// Remove the event from the slice
			s.events = append(s.events[:i], s.events[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("event not found: %s", eventID)
}

// Count returns the total number of stored events
func (s *memoryEventStorage) Count(ctx context.Context) (int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return int64(len(s.events)), nil
}

// DeleteAllEvents removes all events from memory storage
func (s *memoryEventStorage) DeleteAllEvents(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.events = make([]Event, 0)
	return nil
}

// Close closes the storage (no-op for memory storage)
func (s *memoryEventStorage) Close() error {
	return nil
}
