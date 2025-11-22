package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/domain/library"
)

// BrowserHandler handles filesystem browsing requests
type BrowserHandler struct {
	browser library.PathBrowser
}

// NewBrowserHandler creates a new browser handler
func NewBrowserHandler(browser library.PathBrowser) *BrowserHandler {
	return &BrowserHandler{
		browser: browser,
	}
}

// Browse godoc
// @Summary Browse server filesystem
// @Description Browse directories for library path selection
// @Tags filesystem
// @Produce json
// @Param path query string false "Directory path to browse"
// @Success 200 {object} library.BrowseResult
// @Failure 400 {object} ErrorResponse "Invalid path"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Failure 404 {object} ErrorResponse "Directory not found"
// @Router /api/filesystem/browse [get]
func (h *BrowserHandler) Browse(c *gin.Context) {
	path := c.Query("path")

	result, err := h.browser.Browse(c.Request.Context(), path)
	if err != nil {
		// Map domain errors to HTTP status codes
		switch {
		case errors.Is(err, library.ErrInvalidPath),
			errors.Is(err, library.ErrPathTraversal):
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid path",
				Message: err.Error(),
			})
		case errors.Is(err, library.ErrSystemDirectory),
			errors.Is(err, library.ErrOutsideAllowed):
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "Access denied",
				Message: err.Error(),
			})
		case errors.Is(err, library.ErrPermissionDenied):
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "Permission denied",
				Message: err.Error(),
			})
		case errors.Is(err, library.ErrDirectoryNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Directory not found",
				Message: err.Error(),
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Internal server error",
				Message: "Failed to browse directory",
			})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
