package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
)

// EventsHandler handles system event endpoints
type EventsHandler struct {
	eventBus events.EventBus
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(eventBus events.EventBus) *EventsHandler {
	return &EventsHandler{
		eventBus: eventBus,
	}
}

// GetEvents returns system events with filtering and pagination
func (h *EventsHandler) GetEvents(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	eventType := c.Query("type")
	source := c.Query("source")
	priority := c.Query("priority")
	tags := c.QueryArray("tags")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Build filter
	filter := events.EventFilter{}
	
	if eventType != "" {
		filter.Types = []events.EventType{events.EventType(eventType)}
	}
	
	if source != "" {
		filter.Sources = []string{source}
	}
	
	if priority != "" {
		if p, err := strconv.Atoi(priority); err == nil {
			prio := events.EventPriority(p)
			filter.Priority = &prio
		}
	}
	
	if len(tags) > 0 {
		filter.Tags = tags
	}

	// Get events
	eventList, total, err := h.eventBus.GetEvents(filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve events",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": eventList,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetEventsByTimeRange returns events within a specific time range
func (h *EventsHandler) GetEventsByTimeRange(c *gin.Context) {
	// Parse query parameters
	startStr := c.Query("start")
	endStr := c.Query("end")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Both 'start' and 'end' parameters are required (RFC3339 format)",
		})
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid start time format, expected RFC3339",
			"details": err.Error(),
		})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid end time format, expected RFC3339",
			"details": err.Error(),
		})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get events by time range
	eventList, total, err := h.eventBus.GetEventsByTimeRange(start, end, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve events",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": eventList,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"start":  start,
		"end":    end,
	})
}

// GetEventStats returns event bus statistics
func (h *EventsHandler) GetEventStats(c *gin.Context) {
	stats := h.eventBus.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetEventTypes returns available event types
func (h *EventsHandler) GetEventTypes(c *gin.Context) {
	eventTypes := []string{
		string(events.EventMediaLibraryScanned),
		string(events.EventMediaFileFound),
		string(events.EventMediaMetadataEnriched),
		string(events.EventMediaFileDeleted),
		string(events.EventUserCreated),
		string(events.EventUserLoggedIn),
		string(events.EventUserDeviceRegistered),
		string(events.EventPlaybackStarted),
		string(events.EventPlaybackFinished),
		string(events.EventPlaybackProgress),
		string(events.EventSystemStarted),
		string(events.EventSystemStopped),
		string(events.EventPluginLoaded),
		string(events.EventPluginUnloaded),
		string(events.EventPluginEnabled),
		string(events.EventPluginDisabled),
		string(events.EventPluginInstalled),
		string(events.EventPluginError),
		string(events.EventScanStarted),
		string(events.EventScanProgress),
		string(events.EventScanCompleted),
		string(events.EventScanFailed),
		string(events.EventScanResumed),
		string(events.EventScanPaused),
		string(events.EventError),
		string(events.EventWarning),
		string(events.EventInfo),
		string(events.EventDebug),
	}

	c.JSON(http.StatusOK, gin.H{
		"event_types": eventTypes,
		"count":       len(eventTypes),
	})
}

// PublishEvent allows manual event publishing (for testing/admin purposes)
func (h *EventsHandler) PublishEvent(c *gin.Context) {
	var req struct {
		Type     string                 `json:"type" binding:"required"`
		Source   string                 `json:"source" binding:"required"`
		Title    string                 `json:"title"`
		Message  string                 `json:"message"`
		Data     map[string]interface{} `json:"data"`
		Priority int                    `json:"priority"`
		Tags     []string              `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create event
	event := events.NewEvent(
		events.EventType(req.Type),
		req.Source,
		req.Title,
		req.Message,
	)

	if req.Data != nil {
		event.Data = req.Data
	}

	if req.Priority > 0 {
		event.Priority = events.EventPriority(req.Priority)
	}

	if req.Tags != nil {
		event.Tags = req.Tags
	}

	// Publish event
	if err := h.eventBus.Publish(context.Background(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to publish event",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Event published successfully",
		"event_id": event.ID,
	})
}

// GetSubscriptions returns active event subscriptions
func (h *EventsHandler) GetSubscriptions(c *gin.Context) {
	subscriptions := h.eventBus.GetSubscriptions()
	
	// Convert to response format (without the handler function)
	var response []gin.H
	for _, sub := range subscriptions {
		response = append(response, gin.H{
			"id":             sub.ID,
			"filter":         sub.Filter,
			"subscriber":     sub.Subscriber,
			"created":        sub.Created,
			"last_triggered": sub.LastTriggered,
			"trigger_count":  sub.TriggerCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": response,
		"count":         len(response),
	})
}

// ClearEvents removes all events from the system
func (h *EventsHandler) ClearEvents(c *gin.Context) {
	// Create a background context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Clear all events
	if err := h.eventBus.ClearEvents(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to clear events",
			"details": err.Error(),
		})
		return
	}
	
	// Log that events were cleared (as a special event)
	clearEvent := events.NewSystemEvent(
		events.EventInfo,
		"Events Cleared",
		"All system events have been cleared by administrator",
	)
	if err := h.eventBus.PublishAsync(clearEvent); err != nil {
		// Just log the error, don't fail the request
		c.JSON(http.StatusOK, gin.H{
			"message": "All events cleared successfully (but failed to log clear event)",
			"success": true,
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "All events cleared successfully",
		"success": true,
	})
}
