package mediamodule

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
)

// getLibraries returns all media libraries
func (m *Module) getLibraries(c *gin.Context) {
	libraries, err := m.libraryManager.GetAllLibraries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get libraries: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"libraries": libraries,
		"count":     len(libraries),
	})
}

// createLibrary creates a new media library
func (m *Module) createLibrary(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Type string `json:"type" binding:"required"`
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Use provided name or path as name if not provided
	name := req.Name
	if name == "" {
		name = req.Path
	}

	library, err := m.libraryManager.CreateLibrary(name, req.Path, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create library: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Library created successfully",
		"library": library,
	})
}

// deleteLibrary deletes a media library comprehensively
func (m *Module) deleteLibrary(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}

	logger.Info("Deleting library via API", "library_id", id)

	// Use the comprehensive deletion service
	deletionService := NewLibraryDeletionService(m.db, m.eventBus)

	// TODO: Set scanner manager if available when integration is complete
	// if m.scannerManager != nil {
	//     deletionService.SetScannerManager(m.scannerManager)
	// }

	// Perform comprehensive deletion
	result := deletionService.DeleteLibrary(uint32(id))

	if !result.Success {
		logger.Error("Library deletion failed", "library_id", id, "error", result.Error)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   result.Message,
				"details": result.Error.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": result.Message,
			})
		}
		return
	}

	logger.Info("Library deletion completed successfully", "library_id", id, "duration", result.Duration)

	c.JSON(http.StatusOK, gin.H{
		"message":       result.Message,
		"library_id":    result.LibraryID,
		"cleanup_stats": result.CleanupStats,
		"duration":      result.Duration.String(),
	})
}

// getLibraryStats returns statistics for a library
func (m *Module) getLibraryStats(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}

	stats, err := m.libraryManager.GetLibraryStats(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get library stats: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// getLibraryFiles returns all files in a library
func (m *Module) getLibraryFiles(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}

	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var mediaFiles []database.MediaFile
	var total int64

	// Get total count
	m.db.Model(&database.MediaFile{}).Where("library_id = ?", id).Count(&total)

	// Get paginated results
	result := m.db.Where("library_id = ?", id).
		Limit(limit).
		Offset(offset).
		Order("path").
		Find(&mediaFiles)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get media files: %v", result.Error),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}

// getFiles returns all media files
func (m *Module) getFiles(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var mediaFiles []database.MediaFile
	var total int64

	// Get total count
	m.db.Model(&database.MediaFile{}).Count(&total)

	// Get paginated results
	result := m.db.Limit(limit).
		Offset(offset).
		Order("id DESC").
		Find(&mediaFiles)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get media files: %v", result.Error),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}

// getFile returns a specific media file
func (m *Module) getFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_file": mediaFile,
	})
}

// deleteFile deletes a media file
func (m *Module) deleteFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Then delete the file record
	if err := m.db.Delete(&mediaFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete media file: %v", err),
		})
		return
	}

	// TODO: With new schema, metadata deletion would be through Artist/Album/Track relationships
	// For now, just return success since MediaFile deletion is already handled
	// if err := m.db.Where("media_file_id = ?", idStr).Delete(&database.MusicMetadata{}).Error; err != nil {
	//	 return c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete music metadata"})
	// }

	c.JSON(http.StatusOK, gin.H{
		"message": "Media file deleted successfully",
		"id":      idStr,
	})
}

// streamFile streams a media file
func (m *Module) streamFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Check if file exists and get file info
	fileInfo, err := os.Stat(mediaFile.Path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
		})
		return
	}

	// Determine content type
	contentType := getContentTypeFromPath(mediaFile.Path)

	// Set appropriate headers
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-cache")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// For HEAD requests, just return headers without body
	if c.Request.Method == "HEAD" {
		c.Status(http.StatusOK)
		return
	}

	// Serve the file for GET requests
	c.File(mediaFile.Path)
}

// generateHLSManifest generates an HLS manifest for direct video file playback
func (m *Module) generateHLSManifest(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(mediaFile.Path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
		})
		return
	}

	// For now, create a simple HLS manifest that points to the direct stream
	// This is a basic single-bitrate manifest for direct file playback

	// Use relative URL for the stream so it works with any proxy setup
	// This allows the browser to resolve the URL relative to the current origin
	streamURL := fmt.Sprintf("/api/media/files/%s/stream", idStr)

	manifest := fmt.Sprintf(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:999999.0,
%s
#EXT-X-ENDLIST
`, streamURL)

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, manifest)
}

// getFileMetadata retrieves metadata for a media file
func (m *Module) getFileMetadata(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Get the media file with proper relationships
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Based on media type, get the appropriate metadata
	var metadata interface{}

	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		// Get track with artist and album information
		var track database.Track
		if err := m.db.Preload("Artist").Preload("Album").Preload("Album.Artist").
			Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Track metadata not found",
			})
			return
		}
		metadata = map[string]interface{}{
			"type":         "track",
			"track_id":     track.ID,
			"title":        track.Title,
			"track_number": track.TrackNumber,
			"duration":     track.Duration,
			"lyrics":       track.Lyrics,
			"artist": map[string]interface{}{
				"id":          track.Artist.ID,
				"name":        track.Artist.Name,
				"description": track.Artist.Description,
				"image":       track.Artist.Image,
			},
			"album": map[string]interface{}{
				"id":           track.Album.ID,
				"title":        track.Album.Title,
				"release_date": track.Album.ReleaseDate,
				"artwork":      track.Album.Artwork,
			},
		}
	case database.MediaTypeMovie:
		// Get movie information
		var movie database.Movie
		if err := m.db.Where("id = ?", mediaFile.MediaID).First(&movie).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Movie metadata not found",
			})
			return
		}
		metadata = map[string]interface{}{
			"type":                 "movie",
			"movie_id":             movie.ID,
			"title":                movie.Title,
			"original_title":       movie.OriginalTitle,
			"release_date":         movie.ReleaseDate,
			"overview":             movie.Overview,
			"tagline":              movie.Tagline,
			"runtime":              movie.Runtime,
			"rating":               movie.Rating,
			"tmdb_rating":          movie.TmdbRating,
			"vote_count":           movie.VoteCount,
			"popularity":           movie.Popularity,
			"status":               movie.Status,
			"budget":               movie.Budget,
			"revenue":              movie.Revenue,
			"poster":               movie.Poster,
			"backdrop":             movie.Backdrop,
			"tmdb_id":              movie.TmdbID,
			"imdb_id":              movie.ImdbID,
			"adult":                movie.Adult,
			"video":                movie.Video,
			"original_language":    movie.OriginalLanguage,
			"genres":               movie.Genres,
			"production_companies": movie.ProductionCompanies,
			"production_countries": movie.ProductionCountries,
			"spoken_languages":     movie.SpokenLanguages,
			"keywords":             movie.Keywords,
			"main_cast":            movie.MainCast,
			"main_crew":            movie.MainCrew,
			"external_ids":         movie.ExternalIDs,
			"collection":           movie.Collection,
			"awards":               movie.Awards,
		}
	case database.MediaTypeEpisode:
		// Get episode information
		var episode database.Episode
		if err := m.db.Preload("Season").Preload("Season.TVShow").
			Where("id = ?", mediaFile.MediaID).First(&episode).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Episode metadata not found",
			})
			return
		}
		metadata = map[string]interface{}{
			"type":           "episode",
			"episode_id":     episode.ID,
			"title":          episode.Title,
			"episode_number": episode.EpisodeNumber,
			"air_date":       episode.AirDate,
			"description":    episode.Description,
			"duration":       episode.Duration,
			"still_image":    episode.StillImage,
			"season": map[string]interface{}{
				"id":            episode.Season.ID,
				"season_number": episode.Season.SeasonNumber,
				"description":   episode.Season.Description,
				"poster":        episode.Season.Poster,
				"tv_show": map[string]interface{}{
					"id":             episode.Season.TVShow.ID,
					"title":          episode.Season.TVShow.Title,
					"description":    episode.Season.TVShow.Description,
					"first_air_date": episode.Season.TVShow.FirstAirDate,
					"status":         episode.Season.TVShow.Status,
					"poster":         episode.Season.TVShow.Poster,
					"backdrop":       episode.Season.TVShow.Backdrop,
					"tmdb_id":        episode.Season.TVShow.TmdbID,
				},
			},
		}
	default:
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No metadata found for this media type",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_file_id": idStr,
		"metadata":      metadata,
	})
}

// getFileAlbumId returns the album UUID for a media file for the new asset system
func (m *Module) getFileAlbumId(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Get the media file with Track and Album information
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Only handle track media types for now
	if mediaFile.MediaType != database.MediaTypeTrack {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Album artwork only available for music tracks",
		})
		return
	}

	// Get the track to find the album
	var track database.Track
	if err := m.db.Preload("Album").Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Track not found for media file",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_file_id": idStr,
		"album_id":      track.Album.ID,
		"asset_url":     fmt.Sprintf("/api/v1/assets/entity/album/%s/preferred/cover", track.Album.ID),
	})
}

// getFileAlbumArtwork serves album artwork for a media file using the new asset system
func (m *Module) getFileAlbumArtwork(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Get the media file with Track and Album information
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Only handle track media types for now
	if mediaFile.MediaType != database.MediaTypeTrack {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Album artwork only available for music tracks",
		})
		return
	}

	// Get the track to find the album
	var track database.Track
	if err := m.db.Preload("Album").Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Track not found for media file",
		})
		return
	}

	// Get quality parameter
	qualityStr := c.Query("quality")
	quality := 0 // Default to original quality
	if qualityStr != "" {
		if q, err := strconv.Atoi(qualityStr); err == nil && q > 0 && q <= 100 {
			quality = q
		}
	}

	// Try to get assets directly using a simple database query
	var asset struct {
		ID     uuid.UUID `json:"id"`
		Path   string    `json:"path"`
		Format string    `json:"format"`
	}

	// Query the database directly for the preferred cover asset using the real Album.ID
	err := m.db.Table("media_assets").
		Select("id, path, format").
		Where("entity_type = ? AND entity_id = ? AND type = ? AND preferred = ?",
			"album", track.Album.ID, "cover", true).
		First(&asset).Error

	if err != nil {
		// Try any cover asset for this album
		err = m.db.Table("media_assets").
			Select("id, path, format").
			Where("entity_type = ? AND entity_id = ? AND type = ?",
				"album", track.Album.ID, "cover").
			First(&asset).Error
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "No album artwork found",
			"album_id": track.Album.ID,
		})
		return
	}

	// Serve the file directly
	// Get the configured data directory instead of hardcoding Docker path
	dataDir := os.Getenv("VIEWRA_DATA_DIR")
	if dataDir == "" {
		dataDir = "./viewra-data"
	}
	fullPath := filepath.Join(dataDir, "assets", asset.Path)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Artwork file not found on disk",
			"path":  fullPath,
		})
		return
	}

	// Set appropriate headers
	c.Header("Content-Type", asset.Format)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache

	if quality > 0 {
		c.Header("X-Quality", qualityStr)
	}

	// Serve the file
	c.File(fullPath)
}

// Upload functionality has been removed as the app will not support media uploads

// Upload to library functionality has been removed as the app will not support media uploads

// extractMetadata extracts metadata from a media file
func (m *Module) extractMetadata(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Get the media file from database
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	if err := m.metadataManager.ExtractMetadata(&mediaFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to extract metadata: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Metadata extracted successfully",
		"id":      idStr,
	})
}

// updateMetadata updates metadata for a media file
func (m *Module) updateMetadata(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Get the media file to determine its type
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Handle different media types
	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		// Handle track metadata update
		var trackUpdate struct {
			Title       string `json:"title"`
			TrackNumber int    `json:"track_number"`
			Duration    int    `json:"duration"`
			Lyrics      string `json:"lyrics"`
			Artist      string `json:"artist"`
			Album       string `json:"album"`
		}

		if err := c.ShouldBindJSON(&trackUpdate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}

		// Get the track
		var track database.Track
		if err := m.db.Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Track not found",
			})
			return
		}

		// Update track fields
		if trackUpdate.Title != "" {
			track.Title = trackUpdate.Title
		}
		if trackUpdate.TrackNumber > 0 {
			track.TrackNumber = trackUpdate.TrackNumber
		}
		if trackUpdate.Duration > 0 {
			track.Duration = trackUpdate.Duration
		}
		track.Lyrics = trackUpdate.Lyrics

		// Save the updated track
		if err := m.db.Save(&track).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to update track: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Track metadata updated successfully",
			"track_id": track.ID,
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Metadata updates not supported for this media type yet",
		})
		return
	}
}

// processFile processes a media file
func (m *Module) processFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	jobID, err := m.fileProcessor.ProcessFile(idStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to process file: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File processing started",
		"job_id":  jobID,
		"id":      idStr,
	})
}

// getProcessingStatus returns the status of processing jobs
func (m *Module) getProcessingStatus(c *gin.Context) {
	stats := m.fileProcessor.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// getHealth returns the health status of the media module
func (m *Module) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": m.version,
	})
}

// getStatus returns the status of the media module
func (m *Module) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":          m.id,
		"name":        m.name,
		"version":     m.version,
		"core":        m.core,
		"initialized": m.initialized,
	})
}

// getStats returns statistics about the media module
func (m *Module) getStats(c *gin.Context) {
	// Collect stats from components
	processorStats := m.fileProcessor.GetStats()
	metadataStats := m.metadataManager.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"processor_stats": processorStats,
		"metadata_stats":  metadataStats,
	})
}

// getTVShows returns all TV shows with pagination and sorting
func (m *Module) getTVShows(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "24")
	offsetStr := c.DefaultQuery("offset", "0")
	sortField := c.DefaultQuery("sort", "title")
	sortOrder := c.DefaultQuery("order", "asc")
	search := c.Query("search")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Validate sort parameters
	validSortFields := map[string]bool{
		"title":          true,
		"first_air_date": true,
		"status":         true,
		"created_at":     true,
	}
	if !validSortFields[sortField] {
		sortField = "title"
	}

	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	var tvShows []database.TVShow
	var total int64

	// Build query
	query := m.db.Model(&database.TVShow{})

	// Filter out invalid entries (likely people imported as TV shows)
	// Only include entries that have either a description OR an air date
	query = query.Where("(description IS NOT NULL AND description != '') OR (first_air_date IS NOT NULL AND first_air_date != '')")

	// Add search filter if provided
	if search != "" {
		query = query.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(search)+"%")
	}

	// Get total count
	query.Count(&total)

	// Get paginated results with sorting
	result := query.Order(fmt.Sprintf("%s %s", sortField, sortOrder)).
		Limit(limit).
		Offset(offset).
		Find(&tvShows)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get TV shows: %v", result.Error),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tv_shows": tvShows,
		"total":    total,
		"count":    len(tvShows),
		"limit":    limit,
		"offset":   offset,
	})
}

// Helper function to get content type based on file extension
func getContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".webm":
		return "video/webm"
	case ".m4v":
		return "video/x-m4v"
	case ".3gp":
		return "video/3gpp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// transcodeToMP4 transcodes a video file to MP4 format on-the-fly for Shaka Player compatibility
func (m *Module) transcodeToMP4(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	// Parse query parameters
	quality := c.DefaultQuery("quality", "720p") // Default quality

	// Get client IP for tracking
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Get the media file
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(mediaFile.Path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
		})
		return
	}

	// Check if the file is already MP4 with H.264 - if so, stream directly
	if strings.ToLower(mediaFile.Container) == "mp4" &&
		strings.ToLower(mediaFile.VideoCodec) == "h264" {
		// File is already compatible, stream directly
		logger.Info("File already compatible, redirecting to direct stream",
			"file_id", idStr,
			"container", mediaFile.Container,
			"codec", mediaFile.VideoCodec,
			"client_ip", clientIP)
		m.streamFile(c)
		return
	}

	logger.Info("Starting transcoding session",
		"file_id", idStr,
		"source_container", mediaFile.Container,
		"source_codec", mediaFile.VideoCodec,
		"target_quality", quality,
		"client_ip", clientIP,
		"user_agent", userAgent,
		"file_size", mediaFile.SizeBytes,
		"file_path", mediaFile.Path)

	// Add headers to help prevent client timeouts
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	// Add custom headers for debugging
	c.Header("X-Transcode-Session", idStr)
	c.Header("X-Transcode-Quality", quality)

	// Build FFmpeg command for transcoding
	ffmpegArgs := []string{
		"-i", mediaFile.Path, // Input file
		"-c:v", "libx264", // Video codec: H.264
		"-preset", "veryfast", // Fast encoding preset
		"-crf", "23", // Constant Rate Factor (quality)
		"-c:a", "aac", // Audio codec: AAC
		"-ac", "2", // Audio channels: stereo
		"-ar", "44100", // Audio sample rate
		"-movflags", "frag_keyframe+empty_moov", // Enable streaming
		"-f", "mp4", // Output format
		"-avoid_negative_ts", "make_zero", // Fix timestamp issues
		"-fflags", "+genpts", // Generate presentation timestamps
		"-", // Output to stdout
	}

	// Adjust quality settings
	switch quality {
	case "480p":
		ffmpegArgs = append(ffmpegArgs[:4], append([]string{"-vf", "scale=-2:480"}, ffmpegArgs[4:]...)...)
	case "720p":
		ffmpegArgs = append(ffmpegArgs[:4], append([]string{"-vf", "scale=-2:720"}, ffmpegArgs[4:]...)...)
	case "1080p":
		ffmpegArgs = append(ffmpegArgs[:4], append([]string{"-vf", "scale=-2:1080"}, ffmpegArgs[4:]...)...)
	}

	logger.Info("FFmpeg command prepared",
		"file_id", idStr,
		"args", strings.Join(ffmpegArgs, " "))

	// Start FFmpeg process
	cmd := exec.Command("ffmpeg", ffmpegArgs...)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("Failed to create stdout pipe", "file_id", idStr, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transcoding",
		})
		return
	}

	// Get stderr pipe for logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("Failed to create stderr pipe", "file_id", idStr, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transcoding",
		})
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start FFmpeg", "file_id", idStr, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transcoding",
		})
		return
	}

	processStartTime := time.Now()
	logger.Info("FFmpeg process started",
		"file_id", idStr,
		"pid", cmd.Process.Pid,
		"start_time", processStartTime)

	// Handle stderr logging in a goroutine
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
				strings.Contains(line, "warning") || strings.Contains(line, "Warning") {
				logger.Warn("FFmpeg stderr", "file_id", idStr, "message", line)
			} else if strings.Contains(line, "frame=") || strings.Contains(line, "time=") {
				// Progress information (log periodically)
				logger.Debug("FFmpeg progress", "file_id", idStr, "message", line)
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Error("Error reading FFmpeg stderr", "file_id", idStr, "error", err)
		}
	}()

	// Enhanced client disconnect detection
	disconnected := make(chan struct{})
	if cn, ok := c.Writer.(http.CloseNotifier); ok {
		go func() {
			select {
			case <-cn.CloseNotify():
				logger.Warn("Client connection closed (CloseNotifier)",
					"file_id", idStr,
					"client_ip", clientIP,
					"duration", time.Since(processStartTime))
				close(disconnected)
			case <-stderrDone:
				// Process completed normally
			}
		}()
	} else {
		logger.Warn("CloseNotifier not available, cannot detect client disconnect", "file_id", idStr)
	}

	// Handle termination
	go func() {
		<-disconnected
		logger.Info("Terminating transcoding due to client disconnect",
			"file_id", idStr,
			"duration", time.Since(processStartTime))
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				logger.Error("Failed to kill FFmpeg process", "file_id", idStr, "error", err)
			}
		}
	}()

	// Copy transcoded data to response with better monitoring
	buffer := make([]byte, 128*1024) // Increased to 128KB buffer
	totalBytes := int64(0)
	lastLogTime := time.Now()
	chunkCount := 0

	for {
		// Check if client disconnected
		select {
		case <-disconnected:
			logger.Info("Stopping stream due to client disconnect", "file_id", idStr)
			return
		default:
		}

		n, err := stdout.Read(buffer)
		if n > 0 {
			written, writeErr := c.Writer.Write(buffer[:n])
			if writeErr != nil {
				logger.Error("Failed to write to response",
					"file_id", idStr,
					"error", writeErr,
					"bytes_written_so_far", totalBytes)
				break
			}
			totalBytes += int64(written)
			chunkCount++

			// Flush immediately for streaming
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}

			// Log progress every 10MB or 30 seconds
			if totalBytes > 0 && (totalBytes%(10*1024*1024) == 0 || time.Since(lastLogTime) > 30*time.Second) {
				lastLogTime = time.Now()
				logger.Info("Transcoding progress",
					"file_id", idStr,
					"bytes_streamed", totalBytes,
					"chunks_sent", chunkCount,
					"duration", time.Since(processStartTime),
					"avg_speed_mbps", float64(totalBytes)/1024/1024/time.Since(processStartTime).Seconds())
			}
		}

		if err == io.EOF {
			logger.Info("FFmpeg output completed (EOF)", "file_id", idStr)
			break
		}
		if err != nil {
			logger.Error("Error reading from FFmpeg stdout", "file_id", idStr, "error", err)
			break
		}
	}

	// Wait for the command to finish
	cmdErr := cmd.Wait()
	duration := time.Since(processStartTime)

	// Wait for stderr logging to complete
	<-stderrDone

	if cmdErr != nil {
		logger.Error("FFmpeg process failed",
			"file_id", idStr,
			"error", cmdErr,
			"duration", duration,
			"bytes_streamed", totalBytes)
	} else {
		logger.Info("Transcoding session completed successfully",
			"file_id", idStr,
			"bytes_streamed", totalBytes,
			"chunks_sent", chunkCount,
			"duration", duration,
			"avg_speed_mbps", float64(totalBytes)/1024/1024/duration.Seconds(),
			"client_ip", clientIP)
	}
}

// getPlaybackDecision returns playback decision for a specific media file
func (m *Module) getPlaybackDecision(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}

	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Create device profile from request headers
	userAgent := c.GetHeader("User-Agent")

	// Simple playback decision logic (device profile would be used for more complex decisions)
	_ = userAgent // Acknowledge we're getting user agent for future use

	// For now, we'll return a basic decision based on file format
	shouldTranscode := false
	reason := "Media is compatible with client capabilities"

	// Check if transcoding is needed based on container format
	if strings.HasSuffix(strings.ToLower(mediaFile.Path), ".mkv") {
		shouldTranscode = true
		reason = "Container format (MKV) requires transcoding for web playback"
	}

	response := gin.H{
		"should_transcode": shouldTranscode,
		"reason":           reason,
		"media_info": gin.H{
			"id":         mediaFile.ID,
			"container":  getFileExtension(mediaFile.Path),
			"path":       mediaFile.Path,
			"size_bytes": mediaFile.SizeBytes,
		},
	}

	if shouldTranscode {
		response["stream_url"] = fmt.Sprintf("/api/media/files/%s/transcode.mp4", idStr)
	} else {
		response["stream_url"] = fmt.Sprintf("/api/media/files/%s/stream", idStr)
	}

	c.JSON(http.StatusOK, response)
}

// getFileExtension returns the file extension without the dot
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	if len(ext) > 1 {
		return ext[1:] // Remove the leading dot
	}
	return ""
}

// redirectToPlaybackModule redirects streaming requests to the modern PlaybackModule
func (m *Module) redirectToPlaybackModule(c *gin.Context) {
	fileID := c.Param("id")

	// Get the media file to extract the path
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", fileID).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Media file not found",
			"message": "Please use the modern PlaybackModule API at /api/playback/decide for video streaming",
		})
		return
	}

	// Return helpful redirect information
	c.JSON(http.StatusTemporaryRedirect, gin.H{
		"message": "This endpoint now uses DASH/HLS streaming via PlaybackModule",
		"modern_workflow": gin.H{
			"step1": "POST /api/playback/decide with media_path and device_profile",
			"step2": "POST /api/playback/start if transcoding is needed",
			"step3": "Use manifest URLs for DASH/HLS streaming",
		},
		"media_path":      mediaFile.Path,
		"file_id":         fileID,
		"redirect_reason": "Intelligent streaming not available in media module",
	})
}
