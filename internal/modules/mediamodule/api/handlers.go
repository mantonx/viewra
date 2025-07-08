package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for media operations
type Handler struct {
	mediaService   services.MediaService
	libraryManager *library.Manager
}

// NewHandler creates a new API handler
func NewHandler(mediaService services.MediaService, libraryManager *library.Manager) *Handler {
	return &Handler{
		mediaService:   mediaService,
		libraryManager: libraryManager,
	}
}

// GetMediaFiles handles GET /api/media/files
func (h *Handler) GetMediaFiles(c *gin.Context) {
	// Parse query parameters
	var filter types.MediaFilter
	
	// Library ID filter
	if libID := c.Query("library_id"); libID != "" {
		if id, err := strconv.ParseUint(libID, 10, 32); err == nil {
			libraryID := uint32(id)
			filter.LibraryID = &libraryID
		}
	}
	
	// Media type filter
	filter.MediaType = c.Query("media_type")
	
	// Search query
	filter.Search = c.Query("search")
	
	// Pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}
	
	// Media format filters
	filter.VideoCodec = c.Query("video_codec")
	filter.AudioCodec = c.Query("audio_codec")
	filter.Container = c.Query("container")
	filter.Resolution = c.Query("resolution")
	filter.PlaybackMethod = c.Query("playback_method")
	
	// Sorting
	filter.SortBy = c.DefaultQuery("sort_by", "created_at")
	filter.SortDesc = c.Query("sort_order") == "desc"
	
	// Get files
	files, err := h.mediaService.ListFiles(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"count": len(files),
	})
}

// GetMediaFile handles GET /api/media/files/:id
func (h *Handler) GetMediaFile(c *gin.Context) {
	id := c.Param("id")
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	c.JSON(http.StatusOK, file)
}

// GetMediaFileMetadata handles GET /api/media/files/:id/metadata
func (h *Handler) GetMediaFileMetadata(c *gin.Context) {
	id := c.Param("id")
	
	// For now, just return the media file info
	// In the future, this could return enriched metadata
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"file_id":     file.ID,
		"media_type":  file.MediaType,
		"path":        file.Path,
		"container":   file.Container,
		"video_codec": file.VideoCodec,
		"audio_codec": file.AudioCodec,
		"resolution":  file.Resolution,
		"duration":    file.Duration,
		"size_bytes":  file.SizeBytes,
	})
}

// StreamMediaFile handles GET /api/media/files/:id/stream
func (h *Handler) StreamMediaFile(c *gin.Context) {
	id := c.Param("id")
	fmt.Printf("DEBUG: StreamMediaFile called with ID: %s\n", id)
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	// Stream the file directly with range support
	h.streamFileWithRangeSupport(c, file.Path, file.Container)
}

// GetLibraries handles GET /api/admin/media-libraries/
func (h *Handler) GetLibraries(c *gin.Context) {
	libraries, err := h.libraryManager.ListLibraries(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, libraries)
}

// CreateLibrary handles POST /api/admin/media-libraries/
func (h *Handler) CreateLibrary(c *gin.Context) {
	var req database.MediaLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	library := &database.MediaLibrary{
		Path: req.Path,
		Type: req.Type,
	}
	
	if err := h.libraryManager.CreateLibrary(c.Request.Context(), library); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, library)
}

// DeleteLibrary handles DELETE /api/admin/media-libraries/:id
func (h *Handler) DeleteLibrary(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	if err := h.libraryManager.DeleteLibrary(c.Request.Context(), uint32(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusNoContent, nil)
}

// SearchMedia handles GET /api/media/
func (h *Handler) SearchMedia(c *gin.Context) {
	search := c.Query("q")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	
	filter := types.MediaFilter{
		Search: search,
		Limit:  limit,
	}
	
	files, err := h.mediaService.ListFiles(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"results": files,
		"count":   len(files),
	})
}

// GetTVShows handles GET /api/tv/shows
func (h *Handler) GetTVShows(c *gin.Context) {
	// Query TV shows from database
	var shows []database.TVShow
	db := c.MustGet("db").(*gorm.DB)
	
	query := db.Preload("Seasons.Episodes")
	if err := query.Find(&shows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, shows)
}

// GetMovies handles GET /api/movies
func (h *Handler) GetMovies(c *gin.Context) {
	// Query movies from database
	var movies []database.Movie
	db := c.MustGet("db").(*gorm.DB)
	
	if err := db.Find(&movies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, movies)
}

// GetMusic handles GET /api/media/music
func (h *Handler) GetMusic(c *gin.Context) {
	// Get all music files
	filter := types.MediaFilter{
		MediaType: string(database.MediaTypeTrack),
	}
	
	// Apply additional filters from query params
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}
	
	files, err := h.mediaService.ListFiles(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"tracks": files,
		"count":  len(files),
	})
}

// GetAlbumArtwork handles GET /api/media/files/:id/album-artwork
func (h *Handler) GetAlbumArtwork(c *gin.Context) {
	id := c.Param("id")
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	// For now, return a placeholder response
	// In the future, this would check for embedded artwork or album art files
	c.JSON(http.StatusOK, gin.H{
		"file_id": file.ID,
		"artwork_url": "/api/v1/assets/placeholder/album-art",
		"has_artwork": false,
	})
}

// GetAlbumID handles GET /api/media/files/:id/album-id
func (h *Handler) GetAlbumID(c *gin.Context) {
	id := c.Param("id")
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	// Get the track associated with this file
	if file.MediaType == database.MediaTypeTrack && file.MediaID != "" {
		var track database.Track
		db := c.MustGet("db").(*gorm.DB)
		
		if err := db.Where("id = ?", file.MediaID).First(&track).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{
				"album_id": track.AlbumID,
				"track_id": track.ID,
			})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"album_id": nil,
		"track_id": nil,
	})
}

// GetAlbums handles GET /api/music/albums
func (h *Handler) GetAlbums(c *gin.Context) {
	var albums []database.Album
	db := c.MustGet("db").(*gorm.DB)
	
	query := db.Preload("Tracks")
	if err := query.Find(&albums).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, albums)
}

// streamFileWithRangeSupport streams a media file with HTTP range request support
func (h *Handler) streamFileWithRangeSupport(c *gin.Context, filePath, container string) {
	// Security: Ensure the file path is safe and exists
	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}
	
	// Check if file exists
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found on disk"})
		return
	}
	
	// Set content type based on container
	contentType := getContentType(container)
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	
	// For HEAD requests, return only headers
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	
	// Open the file for GET requests
	file, err := os.Open(cleanPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open media file"})
		return
	}
	defer file.Close()
	
	// Handle range requests
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
		h.handleRangeRequest(c, file, fileInfo.Size(), rangeHeader)
		return
	}
	
	// Stream the entire file
	c.Status(http.StatusOK)
	io.Copy(c.Writer, file)
}

// handleRangeRequest handles HTTP range requests for video seeking
func (h *Handler) handleRangeRequest(c *gin.Context, file *os.File, fileSize int64, rangeHeader string) {
	// Parse range header (e.g., "bytes=0-1023" or "bytes=1024-")
	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeParts := strings.Split(rangeSpec, "-")
	
	if len(rangeParts) != 2 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	
	var start, end int64
	var err error
	
	// Parse start byte
	if rangeParts[0] != "" {
		start, err = strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil || start < 0 {
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}
	
	// Parse end byte
	if rangeParts[1] != "" {
		end, err = strconv.ParseInt(rangeParts[1], 10, 64)
		if err != nil || end >= fileSize {
			end = fileSize - 1
		}
	} else {
		end = fileSize - 1
	}
	
	// Validate range
	if start > end || start >= fileSize {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	
	contentLength := end - start + 1
	
	// Set range response headers
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Status(http.StatusPartialContent)
	
	// Seek to start position and copy the requested range
	file.Seek(start, 0)
	io.CopyN(c.Writer, file, contentLength)
}

// getContentType returns the appropriate MIME type for a container format
func getContentType(container string) string {
	switch strings.ToLower(container) {
	case "mp4", "m4v":
		return "video/mp4"
	case "mkv", "matroska":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	case "mov":
		return "video/quicktime"
	case "avi":
		return "video/x-msvideo"
	case "flv":
		return "video/x-flv"
	case "wmv":
		return "video/x-ms-wmv"
	case "mp3":
		return "audio/mpeg"
	case "aac", "m4a":
		return "audio/mp4"
	case "ogg", "oga":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// GetAlbum handles GET /api/music/albums/:id
func (h *Handler) GetAlbum(c *gin.Context) {
	id := c.Param("id")
	
	var album database.Album
	db := c.MustGet("db").(*gorm.DB)
	
	if err := db.Preload("Tracks").Where("id = ?", id).First(&album).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
		return
	}
	
	c.JSON(http.StatusOK, album)
}

// GetArtists handles GET /api/music/artists
func (h *Handler) GetArtists(c *gin.Context) {
	// For now, aggregate artists from tracks
	// In the future, we might have a dedicated Artist table
	db := c.MustGet("db").(*gorm.DB)
	
	var artists []struct {
		Artist string `json:"artist"`
		Count  int    `json:"track_count"`
	}
	
	err := db.Model(&database.Track{}).
		Select("artist, COUNT(*) as count").
		Group("artist").
		Having("artist != ''").
		Order("artist").
		Scan(&artists).Error
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, artists)
}