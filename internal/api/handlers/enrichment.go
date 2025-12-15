package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// EnrichmentHandler handles enrichment-related HTTP requests.
type EnrichmentHandler struct {
	manager  *pipeline.Manager
	eventBus *events.Bus
	logger   *slog.Logger
}

// NewEnrichmentHandler creates a new enrichment handler.
func NewEnrichmentHandler(
	manager *pipeline.Manager,
	eventBus *events.Bus,
	logger *slog.Logger,
) *EnrichmentHandler {
	return &EnrichmentHandler{
		manager:  manager,
		eventBus: eventBus,
		logger:   logger,
	}
}

// GetStats returns enrichment queue statistics.
//
// @Summary Get enrichment queue statistics
// @Description Returns queue statistics for all enrichment stages
// @Tags enrichment
// @Produce json
// @Success 200 {object} map[string]enrichment.QueueStats
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/enrichment/stats [get]
func (h *EnrichmentHandler) GetStats(c *gin.Context) {
	stats, err := h.manager.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get enrichment stats", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get enrichment stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// StreamProgress streams enrichment progress via Server-Sent Events.
// Clients receive real-time updates about enrichment queue status.
//
// @Summary Stream enrichment progress
// @Description Streams enrichment events in real-time via SSE
// @Tags enrichment
// @Produce text/event-stream
// @Param stages query string false "Comma-separated stage names to filter (e.g., 'nfo,tmdb')"
// @Success 200 {string} string "SSE stream"
// @Router /api/enrichment/progress [get]
func (h *EnrichmentHandler) StreamProgress(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Subscribe to enrichment events
	sub := h.eventBus.Subscribe(
		events.WithBufferSize(100),
		events.WithEventPrefix("enrichment."),
		events.WithReplayLast(10), // Replay last 10 enrichment events for late joiners
	)
	defer h.eventBus.Unsubscribe(sub)

	// Create a done channel for cleanup
	clientGone := c.Request.Context().Done()

	// Send initial connection confirmation
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"status\": \"connected\"}\n\n")
	c.Writer.(http.Flusher).Flush()

	for {
		select {
		case <-clientGone:
			// Client disconnected
			h.logger.Debug("SSE client disconnected")
			return
		case event, ok := <-sub.Events():
			if !ok {
				// Subscription closed
				return
			}

			// Convert event to JSON
			eventData := map[string]any{
				"type":      string(event.Type),
				"timestamp": event.Timestamp,
				"source":    event.Source,
				"data":      event.Data,
			}
			if event.RequestID != "" {
				eventData["request_id"] = event.RequestID
			}

			jsonData, err := json.Marshal(eventData)
			if err != nil {
				h.logger.Error("Failed to marshal event", "error", err)
				continue
			}

			// Send SSE event
			fmt.Fprintf(c.Writer, "event: enrichment\ndata: %s\n\n", jsonData)
			c.Writer.(http.Flusher).Flush()
		}
	}
}

// EnqueueMedia manually enqueues a media item for enrichment.
// This can be used to re-process failed items or force enrichment.
//
// @Summary Enqueue media for enrichment
// @Description Manually enqueues a media item for a specific enrichment stage
// @Tags enrichment
// @Accept json
// @Produce json
// @Param request body EnqueueMediaRequest true "Enqueue request"
// @Success 202 {object} EnqueueMediaResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/enrichment/enqueue [post]
func (h *EnrichmentHandler) EnqueueMedia(c *gin.Context) {
	var req EnqueueMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.MediaID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "media_id is required"})
		return
	}

	if req.Stage == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "stage is required"})
		return
	}

	if req.MediaType == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "media_type is required"})
		return
	}

	err := h.manager.EnqueueStage(c.Request.Context(), req.MediaID, enrichment.MediaType(req.MediaType), req.Stage, req.Priority)
	if err != nil {
		h.logger.Error("Failed to enqueue media for enrichment",
			"media_id", req.MediaID,
			"stage", req.Stage,
			"error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to enqueue media"})
		return
	}

	c.JSON(http.StatusAccepted, EnqueueMediaResponse{
		MediaID: req.MediaID,
		Stage:   req.Stage,
		Status:  "queued",
	})
}

// EnqueueMediaRequest represents a request to enqueue media for enrichment.
type EnqueueMediaRequest struct {
	MediaID   int64  `json:"media_id" binding:"required"`
	MediaType string `json:"media_type" binding:"required"` // movie, tv, tv_show, music
	Stage     string `json:"stage" binding:"required"`
	Priority  int    `json:"priority"`
}

// EnqueueMediaResponse represents the response after enqueueing media.
type EnqueueMediaResponse struct {
	MediaID int64  `json:"media_id"`
	Stage   string `json:"stage"`
	Status  string `json:"status"`
}
