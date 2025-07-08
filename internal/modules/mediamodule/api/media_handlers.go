// Package api - Media file handlers
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// GetMediaFiles handles GET /api/media/files
// It returns a paginated list of media files with optional filtering.
//
// Query parameters:
//   - library_id: Filter by library ID
//   - media_type: Filter by type (movie, episode, track)
//   - search: Search in file paths
//   - video_codec: Filter by video codec (h264, hevc, etc)
//   - audio_codec: Filter by audio codec (aac, mp3, etc)
//   - container: Filter by container format (mp4, mkv, etc)
//   - resolution: Filter by resolution (1080p, 720p, etc)
//   - playback_method: Filter by playback compatibility (direct, remux, transcode)
//   - limit: Number of results per page (default: no limit)
//   - offset: Number of results to skip
//   - sort_by: Field to sort by (default: created_at)
//   - sort_order: Sort direction (asc, desc)
//
// Response:
//   {
//     "files": [...],
//     "count": 100
//   }
func (h *Handler) GetMediaFiles(c *gin.Context) {
	// Parse query parameters into filter
	filter := parseMediaFilter(c)
	
	// Get files from service
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
// It returns detailed information about a specific media file.
//
// Path parameters:
//   - id: The media file ID
//
// Response: MediaFile object or 404 if not found
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
// It returns metadata for a specific media file.
// This endpoint can be extended to include enriched metadata from plugins.
//
// Path parameters:
//   - id: The media file ID
//
// Response:
//   {
//     "file_id": "...",
//     "media_type": "movie",
//     "path": "/path/to/file.mp4",
//     "container": "mp4",
//     "video_codec": "h264",
//     "audio_codec": "aac",
//     "resolution": "1920x1080",
//     "duration": 7200,
//     "size_bytes": 1073741824
//   }
func (h *Handler) GetMediaFileMetadata(c *gin.Context) {
	id := c.Param("id")
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	// Basic metadata response
	// TODO: In the future, enrich this with data from metadata plugins
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
		"bitrate_kbps": file.BitrateKbps,
		"created_at":  file.CreatedAt,
		"updated_at":  file.UpdatedAt,
	})
}

// SearchMedia handles GET /api/media/
// It provides a simple search interface for media files.
//
// Query parameters:
//   - q: Search query (searches in file paths)
//   - limit: Maximum results to return (default: 20)
//
// Response:
//   {
//     "results": [...],
//     "count": 10
//   }
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

// GetMusic handles GET /api/media/music
// It returns all music/audio files in the library.
//
// Query parameters:
//   - limit: Number of results per page
//   - offset: Number of results to skip
//
// Response:
//   {
//     "tracks": [...],
//     "count": 100
//   }
func (h *Handler) GetMusic(c *gin.Context) {
	// Parse pagination
	var filter types.MediaFilter
	filter.MediaType = string(database.MediaTypeTrack)
	
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
// It returns album artwork information for a media file.
// Currently returns a placeholder, but can be extended to extract embedded artwork.
//
// Path parameters:
//   - id: The media file ID
//
// Response:
//   {
//     "file_id": "...",
//     "artwork_url": "/api/v1/assets/placeholder/album-art",
//     "has_artwork": false
//   }
func (h *Handler) GetAlbumArtwork(c *gin.Context) {
	id := c.Param("id")
	
	file, err := h.mediaService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}
	
	// TODO: Implement actual artwork extraction from media files
	// This could check for:
	// 1. Embedded artwork in the file
	// 2. Album art files in the same directory (cover.jpg, folder.jpg, etc)
	// 3. Artwork from metadata enrichment plugins
	c.JSON(http.StatusOK, gin.H{
		"file_id": file.ID,
		"artwork_url": "/api/v1/assets/placeholder/album-art",
		"has_artwork": false,
	})
}

// GetAlbumID handles GET /api/media/files/:id/album-id
// It returns the album ID associated with a music file.
//
// Path parameters:
//   - id: The media file ID
//
// Response:
//   {
//     "album_id": "...",
//     "track_id": "..."
//   }
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

// parseMediaFilter extracts filter parameters from the gin context
func parseMediaFilter(c *gin.Context) types.MediaFilter {
	var filter types.MediaFilter
	
	// Library ID filter
	if libID := c.Query("library_id"); libID != "" {
		if id, err := strconv.ParseUint(libID, 10, 32); err == nil {
			libraryID := uint32(id)
			filter.LibraryID = &libraryID
		}
	}
	
	// Basic filters
	filter.MediaType = c.Query("media_type")
	filter.Search = c.Query("search")
	
	// Media format filters
	filter.VideoCodec = c.Query("video_codec")
	filter.AudioCodec = c.Query("audio_codec")
	filter.Container = c.Query("container")
	filter.Resolution = c.Query("resolution")
	filter.PlaybackMethod = c.Query("playback_method")
	
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
	
	// Sorting
	filter.SortBy = c.DefaultQuery("sort_by", "created_at")
	filter.SortDesc = c.Query("sort_order") == "desc"
	
	return filter
}