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
// @Failure 400 {object} APIError "Invalid path"
// @Failure 403 {object} APIError "Access denied"
// @Failure 404 {object} APIError "Directory not found"
// @Router /api/filesystem/browse [get]
func (h *BrowserHandler) Browse(c *gin.Context) {
	path := c.Query("path")

	result, err := h.browser.Browse(c.Request.Context(), path)
	if err != nil {
		// Map domain errors to HTTP status codes
		switch {
		case errors.Is(err, library.ErrInvalidPath),
			errors.Is(err, library.ErrPathTraversal):
			respondError(c, http.StatusBadRequest, "INVALID_PATH", err.Error())
		case errors.Is(err, library.ErrSystemDirectory),
			errors.Is(err, library.ErrOutsideAllowed):
			respondError(c, http.StatusForbidden, "ACCESS_DENIED", err.Error())
		case errors.Is(err, library.ErrPermissionDenied):
			respondError(c, http.StatusForbidden, "PERMISSION_DENIED", err.Error())
		case errors.Is(err, library.ErrDirectoryNotFound):
			respondError(c, http.StatusNotFound, "DIRECTORY_NOT_FOUND", err.Error())
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to browse directory")
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
