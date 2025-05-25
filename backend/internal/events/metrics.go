// Package events provides metrics collection for the event system.
package events

import (
	"sync"
	"time"
)

// basicEventMetrics implements EventMetrics interface
type basicEventMetrics struct {
	mu             sync.RWMutex
	totalEvents    int64
	eventsByType   map[string]int64
	eventsBySource map[string]int64
	eventsByPriority map[string]int64
	subscriptions  int64
	recentEvents   []Event
	maxRecentEvents int
}

// NewBasicEventMetrics creates a new basic event metrics instance
func NewBasicEventMetrics() EventMetrics {
	return &basicEventMetrics{
		eventsByType:     make(map[string]int64),
		eventsBySource:   make(map[string]int64),
		eventsByPriority: make(map[string]int64),
		recentEvents:     make([]Event, 0),
		maxRecentEvents:  100,
	}
}

// RecordEvent records an event for metrics
func (m *basicEventMetrics) RecordEvent(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalEvents++
	
	// Record by type
	m.eventsByType[string(event.Type)]++
	
	// Record by source
	m.eventsBySource[event.Source]++
	
	// Record by priority
	priorityKey := getPriorityKey(event.Priority)
	m.eventsByPriority[priorityKey]++
	
	// Add to recent events
	m.recentEvents = append(m.recentEvents, event)
	if len(m.recentEvents) > m.maxRecentEvents {
		m.recentEvents = m.recentEvents[1:]
	}
}

// RecordSubscription records a subscription event
func (m *basicEventMetrics) RecordSubscription(subscription *Subscription) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.subscriptions++
}

// RecordUnsubscription records an unsubscription event
func (m *basicEventMetrics) RecordUnsubscription(subscriptionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.subscriptions > 0 {
		m.subscriptions--
	}
}

// GetMetrics returns current metrics
func (m *basicEventMetrics) GetMetrics() EventStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Deep copy maps to avoid race conditions
	eventsByType := make(map[string]int64)
	for k, v := range m.eventsByType {
		eventsByType[k] = v
	}
	
	eventsBySource := make(map[string]int64)
	for k, v := range m.eventsBySource {
		eventsBySource[k] = v
	}
	
	eventsByPriority := make(map[string]int64)
	for k, v := range m.eventsByPriority {
		eventsByPriority[k] = v
	}
	
	// Copy recent events
	recentEvents := make([]Event, len(m.recentEvents))
	copy(recentEvents, m.recentEvents)
	
	return EventStats{
		TotalEvents:         m.totalEvents,
		EventsByType:        eventsByType,
		EventsBySource:      eventsBySource,
		EventsByPriority:    eventsByPriority,
		RecentEvents:        recentEvents,
		ActiveSubscriptions: int(m.subscriptions),
	}
}

// getPriorityKey returns a string key for the priority
func getPriorityKey(priority EventPriority) string {
	switch priority {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// advancedEventMetrics provides more detailed metrics with time-based aggregation
type advancedEventMetrics struct {
	basicEventMetrics
	
	// Time-based metrics
	hourlyStats   map[string]*HourlyStats
	dailyStats    map[string]*DailyStats
	startTime     time.Time
}

// HourlyStats represents metrics for an hour
type HourlyStats struct {
	Hour         time.Time        `json:"hour"`
	EventCount   int64            `json:"event_count"`
	EventsByType map[string]int64 `json:"events_by_type"`
}

// DailyStats represents metrics for a day
type DailyStats struct {
	Day          time.Time        `json:"day"`
	EventCount   int64            `json:"event_count"`
	EventsByType map[string]int64 `json:"events_by_type"`
	PeakHour     time.Time        `json:"peak_hour"`
	PeakCount    int64            `json:"peak_count"`
}

// NewAdvancedEventMetrics creates a new advanced event metrics instance
func NewAdvancedEventMetrics() EventMetrics {
	return &advancedEventMetrics{
		basicEventMetrics: basicEventMetrics{
			eventsByType:     make(map[string]int64),
			eventsBySource:   make(map[string]int64),
			eventsByPriority: make(map[string]int64),
			recentEvents:     make([]Event, 0),
			maxRecentEvents:  100,
		},
		hourlyStats: make(map[string]*HourlyStats),
		dailyStats:  make(map[string]*DailyStats),
		startTime:   time.Now(),
	}
}

// RecordEvent records an event for advanced metrics
func (m *advancedEventMetrics) RecordEvent(event Event) {
	// Call basic recording
	m.basicEventMetrics.RecordEvent(event)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Record hourly stats
	hourKey := event.Timestamp.Truncate(time.Hour).Format("2006-01-02T15")
	if m.hourlyStats[hourKey] == nil {
		m.hourlyStats[hourKey] = &HourlyStats{
			Hour:         event.Timestamp.Truncate(time.Hour),
			EventsByType: make(map[string]int64),
		}
	}
	m.hourlyStats[hourKey].EventCount++
	m.hourlyStats[hourKey].EventsByType[string(event.Type)]++
	
	// Record daily stats
	dayKey := event.Timestamp.Truncate(24 * time.Hour).Format("2006-01-02")
	if m.dailyStats[dayKey] == nil {
		m.dailyStats[dayKey] = &DailyStats{
			Day:          event.Timestamp.Truncate(24 * time.Hour),
			EventsByType: make(map[string]int64),
		}
	}
	m.dailyStats[dayKey].EventCount++
	m.dailyStats[dayKey].EventsByType[string(event.Type)]++
	
	// Update peak hour for the day
	if m.hourlyStats[hourKey].EventCount > m.dailyStats[dayKey].PeakCount {
		m.dailyStats[dayKey].PeakHour = event.Timestamp.Truncate(time.Hour)
		m.dailyStats[dayKey].PeakCount = m.hourlyStats[hourKey].EventCount
	}
	
	// Cleanup old stats (keep only last 30 days)
	m.cleanupOldStats()
}

// cleanupOldStats removes stats older than 30 days
func (m *advancedEventMetrics) cleanupOldStats() {
	cutoff := time.Now().AddDate(0, 0, -30)
	
	// Cleanup hourly stats
	for key, stats := range m.hourlyStats {
		if stats.Hour.Before(cutoff) {
			delete(m.hourlyStats, key)
		}
	}
	
	// Cleanup daily stats
	for key, stats := range m.dailyStats {
		if stats.Day.Before(cutoff) {
			delete(m.dailyStats, key)
		}
	}
}

// GetHourlyStats returns hourly statistics
func (m *advancedEventMetrics) GetHourlyStats(hours int) []*HourlyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var stats []*HourlyStats
	now := time.Now()
	
	for i := 0; i < hours; i++ {
		hour := now.Add(-time.Duration(i) * time.Hour).Truncate(time.Hour)
		hourKey := hour.Format("2006-01-02T15")
		
		if stat, exists := m.hourlyStats[hourKey]; exists {
			stats = append(stats, stat)
		} else {
			// Create empty stat for missing hours
			stats = append(stats, &HourlyStats{
				Hour:         hour,
				EventCount:   0,
				EventsByType: make(map[string]int64),
			})
		}
	}
	
	return stats
}

// GetDailyStats returns daily statistics
func (m *advancedEventMetrics) GetDailyStats(days int) []*DailyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var stats []*DailyStats
	now := time.Now()
	
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		dayKey := day.Format("2006-01-02")
		
		if stat, exists := m.dailyStats[dayKey]; exists {
			stats = append(stats, stat)
		} else {
			// Create empty stat for missing days
			stats = append(stats, &DailyStats{
				Day:          day,
				EventCount:   0,
				EventsByType: make(map[string]int64),
			})
		}
	}
	
	return stats
}

// noopEventMetrics is a no-op implementation for when metrics are disabled
type noopEventMetrics struct{}

// NewNoopEventMetrics creates a no-op metrics instance
func NewNoopEventMetrics() EventMetrics {
	return &noopEventMetrics{}
}

func (m *noopEventMetrics) RecordEvent(event Event)                    {}
func (m *noopEventMetrics) RecordSubscription(subscription *Subscription) {}
func (m *noopEventMetrics) RecordUnsubscription(subscriptionID string) {}

func (m *noopEventMetrics) GetMetrics() EventStats {
	return EventStats{
		TotalEvents:         0,
		EventsByType:        make(map[string]int64),
		EventsBySource:      make(map[string]int64),
		EventsByPriority:    make(map[string]int64),
		RecentEvents:        []Event{},
		ActiveSubscriptions: 0,
	}
}
