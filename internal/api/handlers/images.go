package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/images"
)

// ImagesHandler handles HTTP requests for images
type ImagesHandler struct {
	getImage       images.GetImageExecutor
	getMediaImages images.GetMediaImagesExecutor
	getEntityImages images.GetEntityImagesExecutor
}

// NewImagesHandler creates a new images handler
func NewImagesHandler(
	getImage images.GetImageExecutor,
	getMediaImages images.GetMediaImagesExecutor,
	getEntityImages images.GetEntityImagesExecutor,
) *ImagesHandler {
	return &ImagesHandler{
		getImage:        getImage,
		getMediaImages:  getMediaImages,
		getEntityImages: getEntityImages,
	}
}

// GetImage handles GET /api/images/:id
// @Summary Get image metadata by ID
// @Description Returns metadata for a specific image
// @Tags images
// @Produce json
// @Param id path int true "Image ID"
// @Success 200 {object} images.ImageResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/images/{id} [get]
func (h *ImagesHandler) GetImage(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getImage.Execute(c.Request.Context(), int(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Image not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ServeImage handles GET /api/images/:id/file
// @Summary Serve image file
// @Description Serves the actual image file with proper caching headers
// @Tags images
// @Produce image/jpeg,image/png,image/webp
// @Param id path int true "Image ID"
// @Param width query int false "Resize width (preserves aspect ratio)"
// @Param height query int false "Resize height (preserves aspect ratio)"
// @Success 200 {file} binary
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/images/{id}/file [get]
func (h *ImagesHandler) ServeImage(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid ID",
			Message: err.Error(),
		})
		return
	}

	// Get image metadata
	img, err := h.getImage.Execute(c.Request.Context(), int(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Image not found",
			Message: err.Error(),
		})
		return
	}

	// Determine file path
	var filePath string
	if img.FilePath != "" {
		// Local file
		filePath = img.FilePath
	} else if img.LocalCachePath != nil && *img.LocalCachePath != "" {
		// Cached external file
		filePath = *img.LocalCachePath
	} else {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Image file not available",
			Message: "No local file or cache path for this image",
		})
		return
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Image file not found",
			Message: fmt.Sprintf("File does not exist: %s", filepath.Base(filePath)),
		})
		return
	}

	// Set caching headers (1 year for immutable content)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")

	// Set ETag based on file hash
	if img.FileHash != nil {
		c.Header("ETag", fmt.Sprintf(`"%s"`, *img.FileHash))
	}

	// Check resize parameters
	widthStr := c.Query("width")
	heightStr := c.Query("height")

	if widthStr != "" || heightStr != "" {
		// TODO: Phase 4.5 - Implement on-demand resizing
		// For now, serve original
		c.File(filePath)
		return
	}

	// Serve original file
	c.File(filePath)
}

// GetMediaImages handles GET /api/media/:mediaId/images
// @Summary Get all images for a media item
// @Description Returns all images associated with a specific media item
// @Tags images
// @Produce json
// @Param mediaId path int true "Media ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{mediaId}/images [get]
func (h *ImagesHandler) GetMediaImages(c *gin.Context) {
	mediaID, err := parseID(c.Param("mediaId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getMediaImages.Execute(c.Request.Context(), int(mediaID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMovieImages handles GET /api/movies/:id/images
// @Summary Get all images for a movie
// @Description Returns all images (poster, fanart, clearlogo, etc.) for a specific movie
// @Tags movies,images
// @Produce json
// @Param id path int true "Movie media ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/movies/{id}/images [get]
func (h *ImagesHandler) GetMovieImages(c *gin.Context) {
	// Reuse GetMediaImages logic
	c.Params = append(c.Params, gin.Param{Key: "mediaId", Value: c.Param("id")})
	h.GetMediaImages(c)
}

// GetEpisodeImages handles GET /api/tv/episodes/:id/images
// @Summary Get all images for a TV episode
// @Description Returns all images (primarily thumbnails) for a specific episode
// @Tags tv,images
// @Produce json
// @Param id path int true "Episode media ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/episodes/{id}/images [get]
func (h *ImagesHandler) GetEpisodeImages(c *gin.Context) {
	// Reuse GetMediaImages logic
	c.Params = append(c.Params, gin.Param{Key: "mediaId", Value: c.Param("id")})
	h.GetMediaImages(c)
}

// parseID is a helper to parse ID from string to int64
func parseIDInt(idStr string) (int, error) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format")
	}
	return int(id), nil
}
