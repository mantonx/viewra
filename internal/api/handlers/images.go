package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/images"
	infraimages "github.com/viewra/viewra/internal/infrastructure/images"
)

// ImagesHandler handles HTTP requests for images
type ImagesHandler struct {
	getImage        images.GetImageExecutor
	getMediaImages  images.GetMediaImagesExecutor
	getEntityImages images.GetEntityImagesExecutor
	transformer     *infraimages.Transformer
	cacheService    *infraimages.CacheService
}

// NewImagesHandler creates a new images handler
func NewImagesHandler(
	getImage images.GetImageExecutor,
	getMediaImages images.GetMediaImagesExecutor,
	getEntityImages images.GetEntityImagesExecutor,
	transformer *infraimages.Transformer,
	cacheService *infraimages.CacheService,
) *ImagesHandler {
	return &ImagesHandler{
		getImage:        getImage,
		getMediaImages:  getMediaImages,
		getEntityImages: getEntityImages,
		transformer:     transformer,
		cacheService:    cacheService,
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
// @Description Serves pre-generated image presets. All images are in WebP format with multiple sizes available.
// @Description Available presets: thumb, medium, large, xlarge (depending on image type).
// @Description If no preset is specified, serves the medium size by default.
// @Tags images
// @Produce image/webp
// @Param id path int true "Image ID"
// @Param preset query string false "Preset name (thumb, medium, large, xlarge)"
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

	// Get file hash (required for constructing preset paths)
	if img.FileHash == nil || *img.FileHash == "" {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Image unavailable",
			Message: "Image hash not available",
		})
		return
	}

	// Get requested preset (default to "medium")
	preset := c.DefaultQuery("preset", "medium")

	// Validate preset name
	validPresets := map[string]bool{
		"thumb":  true,
		"medium": true,
		"large":  true,
		"xlarge": true,
	}
	if !validPresets[preset] {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid preset",
			Message: fmt.Sprintf("Preset must be one of: thumb, medium, large, xlarge (got: %s)", preset),
		})
		return
	}

	// Construct cache path for requested preset
	// Format: {first2}/{next2}/{hash}_{imageType}_{preset}.webp
	hash := *img.FileHash
	if len(hash) < 4 {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Invalid image hash",
			Message: "Image hash too short",
		})
		return
	}

	level1 := hash[0:2]
	level2 := hash[2:4]
	filename := fmt.Sprintf("%s_%s_%s.webp", hash, img.ImageType, preset)
	relativePath := filepath.Join(level1, level2, filename)
	presetPath := h.cacheService.GetCachedPath(relativePath)

	// Check if preset file exists
	if _, err := os.Stat(presetPath); os.IsNotExist(err) {
		// Preset not found - try fallback to original file path
		var fallbackPath string
		if img.FilePath != "" {
			fallbackPath = img.FilePath
		} else if img.LocalCachePath != nil && *img.LocalCachePath != "" {
			fallbackPath = h.cacheService.GetCachedPath(*img.LocalCachePath)
		}

		if fallbackPath != "" {
			if _, err := os.Stat(fallbackPath); err == nil {
				// Serve the original file as fallback
				c.File(fallbackPath)
				return
			}
		}

		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Preset not found",
			Message: fmt.Sprintf("Preset '%s' not generated for this image. Available presets are generated during library scan.", preset),
		})
		return
	}

	// Set caching headers (1 year for immutable content)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", fmt.Sprintf(`"%s-%s"`, hash, preset))
	c.Header("Content-Type", "image/webp")

	// Serve preset file
	c.File(presetPath)
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
