package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/sse"
	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	enrichmentRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/enrichment"
)

// EnrichmentHandler handles enrichment-related HTTP requests.
type EnrichmentHandler struct {
	manager    *pipeline.Manager
	statusRepo *enrichmentRepo.StatusRepository
	queueRepo  *enrichmentRepo.QueueRepository
	eventBus   *events.Bus
	logger     *slog.Logger
}

// NewEnrichmentHandler creates a new enrichment handler.
func NewEnrichmentHandler(
	manager *pipeline.Manager,
	statusRepo *enrichmentRepo.StatusRepository,
	queueRepo *enrichmentRepo.QueueRepository,
	eventBus *events.Bus,
	logger *slog.Logger,
) *EnrichmentHandler {
	return &EnrichmentHandler{
		manager:    manager,
		statusRepo: statusRepo,
		queueRepo:  queueRepo,
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

// CurrentItemResponse represents the currently processing enrichment item.
type CurrentItemResponse struct {
	MediaID   int64  `json:"media_id"`
	MediaType string `json:"media_type"`
	Stage     string `json:"stage"`
	Title     string `json:"title"`
}

// OverallProgressResponse represents unique item-based progress (not inflated by stages).
type OverallProgressResponse struct {
	TotalItems     int64   `json:"total_items"`
	CompletedItems int64   `json:"completed_items"`
	RemainingItems int64   `json:"remaining_items"`
	Percentage     float64 `json:"percentage"`
}

// LibraryEnrichmentProgressResponse represents enrichment progress for a library.
type LibraryEnrichmentProgressResponse struct {
	LibraryID       int64                             `json:"library_id"`
	StageProgress   map[string]*enrichment.QueueStats `json:"stage_progress"`
	TotalPending    int64                             `json:"total_pending"`
	TotalProcessing int64                             `json:"total_processing"`
	TotalCompleted  int64                             `json:"total_completed"`
	TotalFailed     int64                             `json:"total_failed"`
	IsActive        bool                              `json:"is_active"`
	CurrentItem     *CurrentItemResponse              `json:"current_item,omitempty"`
	OverallProgress *OverallProgressResponse          `json:"overall_progress,omitempty"`
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

	// Get currently processing item
	currentItem, err := h.queueRepo.GetCurrentItem(c.Request.Context(), libraryID)
	if err != nil {
		h.logger.Error("Failed to get current enrichment item", "library_id", libraryID, "error", err)
		// Continue without current item - not critical
	}

	// Get overall progress (unique item count, not inflated by stages)
	overallProgress, err := h.statusRepo.GetOverallProgress(c.Request.Context(), libraryID)
	if err != nil {
		h.logger.Error("Failed to get overall enrichment progress", "library_id", libraryID, "error", err)
		// Continue without overall progress - not critical
	}

	c.JSON(http.StatusOK, h.buildProgressResponse(libraryID, progress, currentItem, overallProgress))
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

	sse.SetHeaders(c)

	// Send initial state immediately
	progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
	if err == nil {
		currentItem, _ := h.queueRepo.GetCurrentItem(c.Request.Context(), libraryID)
		overallProgress, _ := h.statusRepo.GetOverallProgress(c.Request.Context(), libraryID)
		initialData := h.buildProgressResponse(libraryID, progress, currentItem, overallProgress)
		jsonData, _ := json.Marshal(initialData)
		fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
		sse.Flush(c)
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
			eventLibraryID, ok := sse.GetInt64FromEvent(event, "library_id")
			if !ok || eventLibraryID != libraryID {
				continue
			}

			// Check if client disconnected before fetching data
			if c.Request.Context().Err() != nil {
				return
			}

			// Fetch fresh progress data and send it (frontend expects full progress response)
			progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
			if err != nil {
				// Don't log context canceled - it's normal when client disconnects
				if c.Request.Context().Err() != nil {
					return
				}
				h.logger.Error("Failed to get library progress for SSE update", "library_id", libraryID, "error", err)
				continue
			}
			currentItem, _ := h.queueRepo.GetCurrentItem(c.Request.Context(), libraryID)
			overallProgress, _ := h.statusRepo.GetOverallProgress(c.Request.Context(), libraryID)
			updateData := h.buildProgressResponse(libraryID, progress, currentItem, overallProgress)
			jsonData, _ := json.Marshal(updateData)
			fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", jsonData)
			sse.Flush(c)
		}
	}
}

func (h *EnrichmentHandler) buildProgressResponse(libraryID int64, progress map[string]*enrichment.QueueStats, currentItem *enrichmentRepo.CurrentEnrichmentItem, overallProgress *enrichmentRepo.OverallProgress) *LibraryEnrichmentProgressResponse {
	var totalPending, totalProcessing, totalCompleted, totalFailed int64
	for _, stats := range progress {
		totalPending += stats.PendingCount
		totalProcessing += stats.ProcessingCount
		totalCompleted += stats.CompletedCount
		totalFailed += stats.FailedCount
	}

	var currentItemResp *CurrentItemResponse
	if currentItem != nil {
		currentItemResp = &CurrentItemResponse{
			MediaID:   currentItem.MediaID,
			MediaType: string(currentItem.MediaType),
			Stage:     currentItem.Stage,
			Title:     currentItem.Title,
		}
	}

	var overallProgressResp *OverallProgressResponse
	if overallProgress != nil && overallProgress.TotalItems > 0 {
		percentage := float64(overallProgress.CompletedItems) / float64(overallProgress.TotalItems) * 100
		overallProgressResp = &OverallProgressResponse{
			TotalItems:     overallProgress.TotalItems,
			CompletedItems: overallProgress.CompletedItems,
			RemainingItems: overallProgress.RemainingItems,
			Percentage:     percentage,
		}
	}

	return &LibraryEnrichmentProgressResponse{
		LibraryID:       libraryID,
		StageProgress:   progress,
		TotalPending:    totalPending,
		TotalProcessing: totalProcessing,
		TotalCompleted:  totalCompleted,
		TotalFailed:     totalFailed,
		IsActive:        totalPending > 0 || totalProcessing > 0,
		CurrentItem:     currentItemResp,
		OverallProgress: overallProgressResp,
	}
}
