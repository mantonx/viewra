// File: /home/fictional/Projects/viewra/backend/internal/server/handlers/event_stream.go
package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
)

// EventStream handles server-sent events streaming for real-time event updates
func (h *EventsHandler) EventStream(c *gin.Context) {
	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for events
	eventChan := make(chan events.Event, 10)

	// Parse filter parameters from the request
	filter := events.EventFilter{}

	// Extract event types filter (comma-separated list of event types)
	if eventTypes := c.Query("types"); eventTypes != "" {
		typeList := parseCSVParam(eventTypes)
		for _, t := range typeList {
			// Convert string to EventType
			filter.Types = append(filter.Types, events.EventType(t))
		}
	}

	// Extract sources filter (comma-separated list of sources)
	if sources := c.Query("sources"); sources != "" {
		filter.Sources = parseCSVParam(sources)
	}

	// Extract tags filter (comma-separated list of tags)
	if tags := c.Query("tags"); tags != "" {
		filter.Tags = parseCSVParam(tags)
	}

	// Extract priority filter
	if priorityStr := c.Query("priority"); priorityStr != "" {
		var priority events.EventPriority
		switch priorityStr {
		case "low":
			priority = events.PriorityLow
		case "normal":
			priority = events.PriorityNormal
		case "high":
			priority = events.PriorityHigh
		case "critical":
			priority = events.PriorityCritical
		default:
			// Invalid priority, use default (no filter)
		}

		if priority > 0 {
			filter.Priority = &priority
		}
	}

	// Create a context with timeout for subscription
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Subscribe to events
	subscription, err := h.eventBus.Subscribe(ctx, filter, func(e events.Event) error {
		select {
		case eventChan <- e:
			// Event sent to channel
		default:
			// Channel buffer full, discard event
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to subscribe to event stream",
		})
		return
	}

	// Send connection confirmation
	c.SSEvent("", gin.H{
		"type":    "connected",
		"message": "Connected to event stream",
		"time":    time.Now(),
	})
	c.Writer.Flush()

	// Close subscription when client disconnects
	go func() {
		<-c.Request.Context().Done()
		h.eventBus.Unsubscribe(subscription.ID)
		close(eventChan)
	}()

	// Stream events to client
	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return false
			}

			// Send event to client
			c.SSEvent("", gin.H{
				"type": "event",
				"data": event,
				"time": time.Now(),
			})
			return true

		case <-time.After(30 * time.Second):
			// Send heartbeat to keep connection alive
			c.SSEvent("", gin.H{
				"type":    "heartbeat",
				"message": "keepalive",
				"time":    time.Now(),
			})
			return true
		}
	})
}

// Helper function to parse comma-separated values
func parseCSVParam(param string) []string {
	if param == "" {
		return []string{}
	}

	// Split by comma and trim whitespace
	var result []string
	for _, v := range strings.Split(param, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
