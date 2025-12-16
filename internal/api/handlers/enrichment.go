package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	enrichmentRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/enrichment"
)

// EnrichmentHandler handles enrichment-related HTTP requests.
type EnrichmentHandler struct {
	manager    *pipeline.Manager
	statusRepo *enrichmentRepo.StatusRepository
	eventBus   *events.Bus
	logger     *slog.Logger
}

// NewEnrichmentHandler creates a new enrichment handler.
func NewEnrichmentHandler(
	manager *pipeline.Manager,
	statusRepo *enrichmentRepo.StatusRepository,
	eventBus *events.Bus,
	logger *slog.Logger,
) *EnrichmentHandler {
	return &EnrichmentHandler{
		manager:    manager,
		statusRepo: statusRepo,
		eventBus:   eventBus,
		logger:     logger,
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

	if req.LibraryID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "library_id is required"})
		return
	}

	err := h.manager.EnqueueStage(c.Request.Context(), req.MediaID, req.LibraryID, enrichment.MediaType(req.MediaType), req.Stage, req.Priority)
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
	LibraryID int64  `json:"library_id" binding:"required"` // Library containing the media
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

// LibraryEnrichmentProgressResponse represents enrichment progress for a library.
type LibraryEnrichmentProgressResponse struct {
	LibraryID       int64                           `json:"library_id"`
	StageProgress   map[string]*enrichment.QueueStats `json:"stage_progress"`
	TotalPending    int64                           `json:"total_pending"`
	TotalProcessing int64                           `json:"total_processing"`
	TotalCompleted  int64                           `json:"total_completed"`
	TotalFailed     int64                           `json:"total_failed"`
	IsActive        bool                            `json:"is_active"`
}

// GetLibraryProgress returns current enrichment progress snapshot for a library.
//
// @Summary Get library enrichment progress
// @Description Returns enrichment progress for a specific library
// @Tags enrichment
// @Produce json
// @Param id path int true "Library ID"
// @Success 200 {object} LibraryEnrichmentProgressResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/libraries/{id}/enrichment/progress [get]
func (h *EnrichmentHandler) GetLibraryProgress(c *gin.Context) {
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid library ID"})
		return
	}

	progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
	if err != nil {
		h.logger.Error("Failed to get library enrichment progress", "library_id", libraryID, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get progress"})
		return
	}

	c.JSON(http.StatusOK, h.buildProgressResponse(libraryID, progress))
}

// StreamLibraryProgress streams enrichment progress for a specific library via SSE.
//
// @Summary Stream library enrichment progress
// @Description Streams enrichment events for a specific library in real-time via SSE
// @Tags enrichment
// @Produce text/event-stream
// @Param id path int true "Library ID"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} handlers.ErrorResponse
// @Router /api/libraries/{id}/enrichment/stream [get]
func (h *EnrichmentHandler) StreamLibraryProgress(c *gin.Context) {
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid library ID"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Send initial state immediately
	progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
	if err == nil {
		initialData := h.buildProgressResponse(libraryID, progress)
		jsonData, _ := json.Marshal(initialData)
		fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
		c.Writer.(http.Flusher).Flush()
	}

	// Subscribe to enrichment events
	sub := h.eventBus.Subscribe(
		events.WithBufferSize(100),
		events.WithEventPrefix("enrichment."),
	)
	defer h.eventBus.Unsubscribe(sub)

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}

			// Filter events by library ID
			// Handle both int64 (direct) and float64 (from JSON unmarshal) types
			var eventLibraryID int64
			switch v := event.Data["library_id"].(type) {
			case int64:
				eventLibraryID = v
			case float64:
				eventLibraryID = int64(v)
			case int:
				eventLibraryID = int64(v)
			default:
				// Skip events without valid library_id
				continue
			}
			if eventLibraryID != libraryID {
				continue
			}

			// Fetch fresh progress data and send it (frontend expects full progress response)
			progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
			if err != nil {
				h.logger.Error("Failed to get library progress for SSE update", "library_id", libraryID, "error", err)
				continue
			}
			updateData := h.buildProgressResponse(libraryID, progress)
			jsonData, _ := json.Marshal(updateData)
			fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", jsonData)
			c.Writer.(http.Flusher).Flush()
		}
	}
}

func (h *EnrichmentHandler) buildProgressResponse(libraryID int64, progress map[string]*enrichment.QueueStats) *LibraryEnrichmentProgressResponse {
	var totalPending, totalProcessing, totalCompleted, totalFailed int64
	for _, stats := range progress {
		totalPending += stats.PendingCount
		totalProcessing += stats.ProcessingCount
		totalCompleted += stats.CompletedCount
		totalFailed += stats.FailedCount
	}

	return &LibraryEnrichmentProgressResponse{
		LibraryID:       libraryID,
		StageProgress:   progress,
		TotalPending:    totalPending,
		TotalProcessing: totalProcessing,
		TotalCompleted:  totalCompleted,
		TotalFailed:     totalFailed,
		IsActive:        totalPending > 0 || totalProcessing > 0,
	}
}
