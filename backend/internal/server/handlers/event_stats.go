// Package handlers provides HTTP handlers for the API server
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
)

// GetEventStats returns accurate event statistics
// This is the updated version that ensures consistency between displayed events and stats
func (h *EventsHandler) GetEventStats(c *gin.Context) {
	// Create an empty filter (correctly typed)
	emptyFilter := events.EventFilter{}                      // Empty filter to get all events
	_, total, err := h.eventBus.GetEvents(emptyFilter, 0, 0) // 0 limit means we only care about the total count

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve event statistics",
			"details": err.Error(),
		})
		return
	}

	// Get other stats from the event bus
	baseStats := h.eventBus.GetStats()

	// Update the total to the accurate count
	baseStats.TotalEvents = total

	// Return the updated stats
	c.JSON(http.StatusOK, baseStats)
}
