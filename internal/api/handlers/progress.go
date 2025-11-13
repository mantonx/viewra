package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/progress"
	progressDomain "github.com/viewra/viewra/internal/domain/progress"
)

// ProgressHandler handles watch progress-related HTTP requests.
type ProgressHandler struct {
	repo progressDomain.Repository
}

// NewProgressHandler creates a new progress handler.
func NewProgressHandler(repo progressDomain.Repository) *ProgressHandler {
	return &ProgressHandler{
		repo: repo,
	}
}

// UpdateProgress updates watch progress for a media item.
//
// @Summary Update watch progress
// @Description Updates or creates watch progress for a media item. Automatically marks as watched if progress exceeds 90%.
// @Tags progress
// @Accept json
// @Produce json
// @Param request body progress.UpdateProgressRequest true "Update progress request"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress [put]
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

	response, err := progress.UpdateProgress(c.Request.Context(), h.repo, &req)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID,
			progressDomain.ErrInvalidProgress,
			progressDomain.ErrInvalidDuration,
			progressDomain.ErrProgressExceedsDuration:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update progress"})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetProgress retrieves watch progress for a media item.
//
// @Summary Get watch progress
// @Description Gets watch progress for a specific media item
// @Tags progress
// @Produce json
// @Param media_id path int true "Media ID"
// @Success 200 {object} progress.WatchProgressResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/{media_id} [get]
func (h *ProgressHandler) GetProgress(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	userID := getCurrentUserID()

	response, err := progress.GetProgressByMediaIDAndUserID(c.Request.Context(), h.repo, mediaID, userID)
	if err != nil {
		if err == progressDomain.ErrProgressNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Progress not found"})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get progress"})
		}
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
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress [get]
func (h *ProgressHandler) ListProgress(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := progress.ListProgressByUserID(c.Request.Context(), h.repo, userID, params.limit, params.offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list progress"})
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
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/watched [get]
func (h *ProgressHandler) ListWatched(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := progress.ListWatchedByUserID(c.Request.Context(), h.repo, userID, params.limit, params.offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list watched items"})
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
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/in-progress [get]
func (h *ProgressHandler) ListInProgress(c *gin.Context) {
	userID := getCurrentUserID()
	params := parsePaginationParams(c)

	response, err := progress.ListInProgressByUserID(c.Request.Context(), h.repo, userID, params.limit, params.offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list in-progress items"})
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
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/mark-watched [post]
func (h *ProgressHandler) MarkWatched(c *gin.Context) {
	var req progress.MarkWatchedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Default to user_id = 1 for now (single-user mode)
	if req.UserID == 0 {
		req.UserID = 1
	}

	response, err := progress.MarkWatched(c.Request.Context(), h.repo, &req)
	if err != nil {
		if err == progressDomain.ErrInvalidMediaID {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to mark as watched"})
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
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/mark-unwatched [post]
func (h *ProgressHandler) MarkUnwatched(c *gin.Context) {
	var req progress.MarkWatchedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Default to user_id = 1 for now (single-user mode)
	if req.UserID == 0 {
		req.UserID = 1
	}

	response, err := progress.MarkUnwatched(c.Request.Context(), h.repo, &req)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case progressDomain.ErrProgressNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Progress not found"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to mark as unwatched"})
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
// @Param media_id path int true "Media ID"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/progress/{media_id} [delete]
func (h *ProgressHandler) DeleteProgress(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	userID := getCurrentUserID()

	err = progress.DeleteProgress(c.Request.Context(), h.repo, mediaID, userID)
	if err != nil {
		switch err {
		case progressDomain.ErrInvalidMediaID:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case progressDomain.ErrProgressNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Progress not found"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete progress"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}
