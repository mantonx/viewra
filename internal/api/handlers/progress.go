package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/progress"
	progressDomain "github.com/mantonx/viewra/internal/domain/progress"
)

// ProgressHandler handles watch progress-related HTTP requests.
type ProgressHandler struct {
	service *progress.Service
}

// NewProgressHandler creates a new progress handler.
func NewProgressHandler(service *progress.Service) *ProgressHandler {
	return &ProgressHandler{
		service: service,
	}
}

// UpdateProgress updates watch progress for a media item.
//
// @Summary Update watch progress
// @Description Updates or creates watch progress for a media item. Automatically marks as watched if progress exceeds 90%. Supports both PUT and POST for browser sendBeacon compatibility.
// @Tags progress
// @Accept json
// @Produce json
// @Param request body progress.UpdateProgressRequest true "Update progress request"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress [put]
// @Router /api/progress [post]
func (h *ProgressHandler) UpdateProgress(c *gin.Context) {
	var req progress.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
		})
		return
	}

	// Default to user_id = 1 for now (single-user mode)
	if req.UserID == 0 {
		req.UserID = 1
	}

	response, err := h.service.Execute(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID,
			progressDomain.ErrInvalidProgress,
			progressDomain.ErrInvalidDuration,
			progressDomain.ErrProgressExceedsDuration:
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update progress")
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetProgress retrieves watch progress for a media item.
//
// @Summary Get watch progress
// @Description Gets watch progress for a specific media item. If device_profile is provided, returns device-specific playback preferences (quality, audio track, subtitle track) if available.
// @Tags progress
// @Produce json
// @Param id path int true "Media ID"
// @Param device_profile query string false "Device profile hash (e.g., 'chrome-h264-sdr') for device-specific preferences"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 404 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/{id} [get]
func (h *ProgressHandler) GetProgress(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid media ID")
		return
	}

	userID := getCurrentUserID()
	deviceProfile := c.Query("device_profile")

	var response *progress.WatchProgressResponse
	if deviceProfile != "" {
		response, err = h.service.GetProgressWithDeviceProfile(c.Request.Context(), mediaID, userID, deviceProfile)
	} else {
		response, err = h.service.GetProgress(c.Request.Context(), mediaID, userID)
	}

	if err != nil {
		if err == progressDomain.ErrProgressNotFound {
			respondError(c, http.StatusNotFound, "NOT_FOUND", "Progress not found")
		} else {
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get progress")
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBatchProgress retrieves watch progress for multiple media items.
//
// @Summary Get batch watch progress
// @Description Gets watch progress for multiple media items
// @Tags progress
// @Produce json
// @Param media_ids query string true "Comma-separated media IDs"
// @Success 200 {object} progress.BatchProgressResponse
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/batch [get]
func (h *ProgressHandler) GetBatchProgress(c *gin.Context) {
	mediaIDsStr := c.Query("media_ids")
	if mediaIDsStr == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "media_ids query parameter is required")
		return
	}

	mediaIDs, err := parseIDList(mediaIDsStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid media IDs format")
		return
	}

	userID := getCurrentUserID()

	response, err := h.service.GetBatchProgress(c.Request.Context(), mediaIDs, userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get batch progress")
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListProgress retrieves all watch progress for the current user.
//
// @Summary List watch progress
// @Description Gets all watch progress records for the current user
// @Tags progress
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} progress.ListProgressResponse
// @Failure 500 {object} handlers.APIError
// @Router /api/progress [get]
func (h *ProgressHandler) ListProgress(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := h.service.ListProgress(c.Request.Context(), userID, params.limit, params.offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list progress")
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListWatched retrieves all watched items for the current user.
//
// @Summary List watched items
// @Description Gets all watched media items for the current user
// @Tags progress
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} progress.ListProgressResponse
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/watched [get]
func (h *ProgressHandler) ListWatched(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := h.service.ListWatched(c.Request.Context(), userID, params.limit, params.offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list watched items")
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListInProgress retrieves all in-progress items for the current user.
//
// @Summary List in-progress items
// @Description Gets all in-progress (partially watched) media items for the current user
// @Tags progress
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} progress.ListProgressResponse
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/in-progress [get]
func (h *ProgressHandler) ListInProgress(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := h.service.ListInProgress(c.Request.Context(), userID, params.limit, params.offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list in-progress items")
		return
	}

	c.JSON(http.StatusOK, response)
}

// MarkWatched marks a media item as fully watched.
//
// @Summary Mark as watched
// @Description Explicitly marks a media item as fully watched
// @Tags progress
// @Accept json
// @Produce json
// @Param request body progress.MarkWatchedRequest true "Mark watched request"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/mark-watched [post]
func (h *ProgressHandler) MarkWatched(c *gin.Context) {
	var req progress.MarkWatchedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}

	// Default to user_id = 1 for now (single-user mode)
	if req.UserID == 0 {
		req.UserID = 1
	}

	response, err := h.service.MarkWatched(c.Request.Context(), &req)
	if err != nil {
		if err == progressDomain.ErrInvalidMediaID {
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		} else {
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark as watched")
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// MarkUnwatched resets the watched status for a media item.
//
// @Summary Mark as unwatched
// @Description Resets the watched status and progress for a media item
// @Tags progress
// @Accept json
// @Produce json
// @Param request body progress.MarkWatchedRequest true "Mark unwatched request"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 400 {object} handlers.APIError
// @Failure 404 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/mark-unwatched [post]
func (h *ProgressHandler) MarkUnwatched(c *gin.Context) {
	var req progress.MarkWatchedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}

	// Default to user_id = 1 for now (single-user mode)
	if req.UserID == 0 {
		req.UserID = 1
	}

	response, err := h.service.MarkUnwatched(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID:
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		case progressDomain.ErrProgressNotFound:
			respondError(c, http.StatusNotFound, "NOT_FOUND", "Progress not found")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark as unwatched")
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteProgress deletes watch progress for a media item.
//
// @Summary Delete watch progress
// @Description Deletes watch progress for a specific media item
// @Tags progress
// @Param id path int true "Media ID"
// @Success 204
// @Failure 400 {object} handlers.APIError
// @Failure 404 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/progress/{id} [delete]
func (h *ProgressHandler) DeleteProgress(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid media ID")
		return
	}

	userID := getCurrentUserID()

	err = h.service.DeleteProgress(c.Request.Context(), mediaID, userID)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID:
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		case progressDomain.ErrProgressNotFound:
			respondError(c, http.StatusNotFound, "NOT_FOUND", "Progress not found")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete progress")
		}
		return
	}

	c.Status(http.StatusNoContent)
}
