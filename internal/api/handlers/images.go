package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/images"
	domainimages "github.com/mantonx/viewra/internal/domain/images"
	infraimages "github.com/mantonx/viewra/internal/infrastructure/images"
)

// ImagesHandler handles HTTP requests for images
type ImagesHandler struct {
	getImage           images.GetImageExecutor
	getMediaImages     images.GetMediaImagesExecutor
	getEntityImages    images.GetEntityImagesExecutor
	getBatchImages     images.GetBatchMediaImagesExecutor
	transformer        *infraimages.Transformer
	cacheService       *infraimages.CacheService
}

// NewImagesHandler creates a new images handler
func NewImagesHandler(
	getImage images.GetImageExecutor,
	getMediaImages images.GetMediaImagesExecutor,
	getEntityImages images.GetEntityImagesExecutor,
	getBatchImages images.GetBatchMediaImagesExecutor,
	transformer *infraimages.Transformer,
	cacheService *infraimages.CacheService,
) *ImagesHandler {
	return &ImagesHandler{
		getImage:           getImage,
		getMediaImages:     getMediaImages,
		getEntityImages:    getEntityImages,
		getBatchImages:     getBatchImages,
		transformer:        transformer,
		cacheService:       cacheService,
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

	// Get preset path from cache service
	hash := *img.FileHash
	presetPath, err := h.cacheService.GetPresetPath(hash, img.ImageType, preset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Invalid image hash",
			Message: err.Error(),
		})
		return
	}

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
// @Param id path int true "Media ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{id}/images [get]
func (h *ImagesHandler) GetMediaImages(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
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

// GetTVShowImages handles GET /api/tv/shows/:id/images
// @Summary Get all images for a TV show
// @Description Returns all images (poster, fanart, clearlogo, banner, etc.) for a specific TV show
// @Tags tv,images
// @Produce json
// @Param id path int true "TV Show ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows/{id}/images [get]
func (h *ImagesHandler) GetTVShowImages(c *gin.Context) {
	showID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid show ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getEntityImages.Execute(c.Request.Context(), domainimages.MediaTypeTVShow, int(showID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTVSeasonImages handles GET /api/tv/seasons/:id/images
// @Summary Get all images for a TV season
// @Description Returns all images (primarily season posters) for a specific TV season
// @Tags tv,images
// @Produce json
// @Param id path int true "TV Season ID"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/seasons/{id}/images [get]
func (h *ImagesHandler) GetTVSeasonImages(c *gin.Context) {
	seasonID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid season ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getEntityImages.Execute(c.Request.Context(), domainimages.MediaTypeTVSeason, int(seasonID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMusicAlbumImages handles GET /api/music/albums/:id/images
// @Summary Get all images for a music album
// @Description Returns all images (cover, discart, etc.) for a specific music album
// @Tags music,images
// @Produce json
// @Param id path int true "Album entity ID (track media_id)"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/albums/{id}/images [get]
func (h *ImagesHandler) GetMusicAlbumImages(c *gin.Context) {
	albumID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid album ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getEntityImages.Execute(c.Request.Context(), domainimages.MediaTypeMusicAlbum, int(albumID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMusicArtistImages handles GET /api/music/artists/:id/images
// @Summary Get all images for a music artist
// @Description Returns all images (folder, fanart, banner, logo) for a specific music artist
// @Tags music,images
// @Produce json
// @Param id path int true "Artist entity ID (first track's media_id)"
// @Success 200 {object} images.ListImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/artists/{id}/images [get]
func (h *ImagesHandler) GetMusicArtistImages(c *gin.Context) {
	artistID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid artist ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getEntityImages.Execute(c.Request.Context(), domainimages.MediaTypeMusicArtist, int(artistID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetBatchMediaImages handles POST /api/images/batch
// @Summary Get images for multiple media items or entities
// @Description Returns images for multiple media items (movies, episodes) or entities (TV shows, artists) in a single request to reduce N+1 queries
// @Tags images
// @Accept json
// @Produce json
// @Param request body object{media_ids=[]int,entity_ids=[]int,media_type=string} true "Batch request with either media_ids or (entity_ids + media_type)"
// @Success 200 {object} images.BatchImagesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/images/batch [post]
func (h *ImagesHandler) GetBatchMediaImages(c *gin.Context) {
	var req struct {
		MediaIDs  []int  `json:"media_ids"`
		EntityIDs []int  `json:"entity_ids"`
		MediaType string `json:"media_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Validate that we have either media_ids OR (entity_ids + media_type)
	hasMediaIDs := len(req.MediaIDs) > 0
	hasEntityIDs := len(req.EntityIDs) > 0 && req.MediaType != ""

	if !hasMediaIDs && !hasEntityIDs {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: "Must provide either media_ids or both entity_ids and media_type",
		})
		return
	}

	if hasMediaIDs && hasEntityIDs {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: "Cannot provide both media_ids and entity_ids in the same request",
		})
		return
	}

	// Limit batch size to prevent abuse
	batchSize := len(req.MediaIDs)
	if hasEntityIDs {
		batchSize = len(req.EntityIDs)
	}
	if batchSize > 200 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Batch size too large",
			Message: "Maximum 200 IDs per request",
		})
		return
	}

	var resp *images.BatchImagesResponse
	var err error

	if hasMediaIDs {
		// Media-based batch lookup (movies, episodes, tracks)
		resp, err = h.getBatchImages.Execute(c.Request.Context(), req.MediaIDs, "", nil)
	} else {
		// Entity-based batch lookup (TV shows, music artists, etc.)
		resp, err = h.getBatchImages.Execute(c.Request.Context(), nil, req.MediaType, req.EntityIDs)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve batch images",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// parseID is a helper to parse ID from string to int64
func parseIDInt(idStr string) (int, error) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format")
	}
	return int(id), nil
}
