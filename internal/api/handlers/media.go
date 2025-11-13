package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/media"
)

// MediaHandler handles HTTP requests for media
type MediaHandler struct {
	getMedia  media.GetMediaExecutor
	listMedia media.ListMediaExecutor
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(
	getMedia media.GetMediaExecutor,
	listMedia media.ListMediaExecutor,
) *MediaHandler {
	return &MediaHandler{
		getMedia:  getMedia,
		listMedia: listMedia,
	}
}

// List handles GET /api/media
// @Summary List media items
// @Description Returns a list of media items with optional filtering
// @Tags media
// @Produce json
// @Param library_id query int false "Filter by library ID (omit to list all media)"
// @Param type query string false "Filter by media type (movie, episode, track)"
// @Param limit query int false "Limit number of results" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} media.ListMediaResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media [get]
func (h *MediaHandler) List(c *gin.Context) {
	// Parse library_id - optional
	libraryIDStr := c.Query("library_id")

	var resp media.ListMediaResponse
	var err error

	if libraryIDStr == "" {
		// No library_id specified, list all media across all libraries
		resp, err = h.listMedia.ExecuteAll(c.Request.Context())
	} else {
		// Library_id specified, filter by library
		libraryID, parseErr := parseID(libraryIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid library_id",
				Message: parseErr.Error(),
			})
			return
		}

		// Note: The current use case signature only takes libraryID
		// Additional filtering by type, limit, offset would need to be added to the use case
		resp, err = h.listMedia.Execute(c.Request.Context(), libraryID)
	}

	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/media/:id
// @Summary Get a media item by ID
// @Description Returns details of a specific media item
// @Tags media
// @Produce json
// @Param id path int true "Media ID"
// @Success 200 {object} media.GetMediaResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{id} [get]
func (h *MediaHandler) Get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getMedia.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, media.GetMediaResponse{Media: resp})
}
