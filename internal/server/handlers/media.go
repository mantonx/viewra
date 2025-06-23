// Media handler with event support
package handlers

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// MediaHandler handles media-related API endpoints
type MediaHandler struct {
	eventBus events.EventBus
}

// NewMediaHandler creates a new media handler with event bus
func NewMediaHandler(eventBus events.EventBus) *MediaHandler {
	return &MediaHandler{
		eventBus: eventBus,
	}
}

// GetMedia retrieves all media items with associated user information
func (h *MediaHandler) GetMedia(c *gin.Context) {
	var mediaFiles []database.MediaFile
	db := database.GetDB()

	result := db.Find(&mediaFiles)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve media",
			"details": result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media": mediaFiles,
		"count": len(mediaFiles),
	})
}

// GetMediaByID retrieves a specific media file by ID
func (h *MediaHandler) GetMediaByID(c *gin.Context) {
	mediaIDStr := c.Param("id")
	if mediaIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media ID",
		})
		return
	}

	// Get the media file from database with music metadata
	var mediaFile database.MediaFile
	db := database.GetDB()
	result := db.Preload("MusicMetadata").Where("id = ?", mediaIDStr).First(&mediaFile)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	c.JSON(http.StatusOK, mediaFile)
}

// StreamMedia serves the actual media file content for streaming
func (h *MediaHandler) StreamMedia(c *gin.Context) {
	mediaIDStr := c.Param("id")
	if mediaIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media ID",
		})
		return
	}

	// Get the media file from database
	var mediaFile database.MediaFile
	db := database.GetDB()
	result := db.Where("id = ?", mediaIDStr).First(&mediaFile)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Resolve the file path using the path resolver
	pathResolver := utils.NewPathResolver()
	validPath, err := pathResolver.ResolvePath(mediaFile.Path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
			"path":  mediaFile.Path,
		})
		return
	}

	// Open the resolved file
	file, err := os.Open(validPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open media file",
		})
		return
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get file info",
		})
		return
	}

	// Get content type using utility
	contentType := utils.GetContentType(validPath)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(validPath)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Set headers for streaming
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")

	// Consider it a playback start if it's a media file
	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		// Get user info if available
		userID := uint(0) // Default to 0 for anonymous

		// Publish playback started event
		if h.eventBus != nil {
			// TODO: With new schema, metadata would be retrieved through Artist/Album/Track relationships
			// For now, use basic file info
			title := filepath.Base(mediaFile.Path)

			playEvent := events.NewSystemEvent(
				events.EventPlaybackStarted,
				"Playback Started",
				fmt.Sprintf("Started streaming: %s", title),
			)
			playEvent.Data = map[string]interface{}{
				"mediaId":   mediaIDStr,
				"userId":    userID,
				"timestamp": time.Now().Unix(),
				"title":     title,
				"path":      mediaFile.Path,
			}
			h.eventBus.PublishAsync(playEvent)
		}
	}

	// Stream the file to the client
	c.DataFromReader(http.StatusOK, fileInfo.Size(), contentType, file, nil)
}

// UploadMedia functionality removed as app won't support uploads
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Upload functionality is not supported",
	})
}

// Keep original function-based handlers for backward compatibility
// These will delegate to the struct-based handlers

// GetMedia function-based handler for backward compatibility
func GetMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.GetMedia(c)
}

// UploadMedia function-based handler for backward compatibility
func UploadMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.UploadMedia(c)
}

// StreamMedia function-based handler for backward compatibility
func StreamMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.StreamMedia(c)
}

// GetArtwork serves album artwork for a media file - deprecated, use entity-based asset system instead
func GetArtwork(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":         "This endpoint is deprecated",
		"message":       "Artwork is now managed through the entity-based asset system",
		"suggestion":    "Use /api/v1/assets/entity/{type}/{id}/preferred/cover for album artwork",
		"documentation": "See /api/v1/assets/ endpoints for the new asset management system",
	})
}

// Helper function to check if a file is a music file
func isMusicFile(extension string) bool {
	ext := strings.ToLower(extension)
	musicExtensions := []string{".mp3", ".flac", ".aac", ".ogg", ".wav", ".m4a"}

	for _, e := range musicExtensions {
		if ext == e {
			return true
		}
	}

	return false
}

// LoadTestMusicData loads test music data for development - deprecated
func LoadTestMusicData(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":      "Load test music data functionality is deprecated",
		"message":    "Test data loading is no longer supported",
		"suggestion": "Use real media files and the scanner system instead",
	})
}

// GetMediaFiles retrieves all media files across all libraries with pagination
func GetMediaFiles(c *gin.Context) {
	// Get limit and offset from query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	db := database.GetDB()

	// Define a struct to hold MediaFile with additional metadata info
	type MediaFileWithMetadata struct {
		database.MediaFile
		TrackInfo *database.Track `json:"track_info,omitempty"`
	}

	// Query media files with pagination
	var mediaFiles []database.MediaFile
	result := db.Limit(limit).Offset(offset).Find(&mediaFiles)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve media files",
			"details": result.Error.Error(),
		})
		return
	}

	// Build response with additional metadata for music files
	var filesWithMetadata []MediaFileWithMetadata
	for _, mediaFile := range mediaFiles {
		fileWithMeta := MediaFileWithMetadata{MediaFile: mediaFile}

		// If it's a music track, get track metadata
		if mediaFile.MediaType == database.MediaTypeTrack && mediaFile.MediaID != "" {
			var track database.Track
			if err := db.Preload("Artist").Preload("Album").Preload("Album.Artist").
				Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
				fileWithMeta.TrackInfo = &track
			}
		}

		filesWithMetadata = append(filesWithMetadata, fileWithMeta)
	}

	// Get total count
	var total int64
	db.Model(&database.MediaFile{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"media_files": filesWithMetadata,
		"count":       len(filesWithMetadata),
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetMediaFile retrieves a specific media file by ID with metadata
func GetMediaFile(c *gin.Context) {
	idParam := c.Param("id")

	if idParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	db := database.GetDB()
	var mediaFile database.MediaFile
	result := db.Where("id = ?", idParam).First(&mediaFile)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Try to find by media_id (episode, movie, or track ID)
			result = db.Where("media_id = ?", idParam).First(&mediaFile)
			if result.Error != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Media file not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve media file",
				"details": result.Error.Error(),
			})
			return
		}
	}

	// Build response with metadata if it's a music track
	response := map[string]interface{}{
		"media_file": mediaFile,
	}

	if mediaFile.MediaType == database.MediaTypeTrack && mediaFile.MediaID != "" {
		var track database.Track
		if err := db.Preload("Artist").Preload("Album").Preload("Album.Artist").
			Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
			response["track_info"] = track
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetMediaFileMetadata retrieves metadata for a media file based on its type
func GetMediaFileMetadata(c *gin.Context) {
	idParam := c.Param("id")

	if idParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	db := database.GetDB()
	var mediaFile database.MediaFile
	result := db.Where("id = ?", idParam).First(&mediaFile)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Media file not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve media file",
				"details": result.Error.Error(),
			})
		}
		return
	}

	response := gin.H{
		"media_file": mediaFile,
	}

	// Based on media type, return appropriate metadata
	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		if mediaFile.MediaID != "" {
			var track database.Track
			if err := db.Preload("Artist").Preload("Album").Preload("Album.Artist").
				Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
				response["metadata"] = gin.H{
					"type":        "track",
					"track_id":    track.ID,
					"title":       track.Title,
					"artist":      track.Artist.Name,
					"album":       track.Album.Title,
					"album_artist": track.Album.Artist.Name,
					"track_number": track.TrackNumber,

					"duration":     track.Duration,
				}
			} else {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Track metadata not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "File is not a music track",
			})
			return
		}
	case database.MediaTypeEpisode:
		if mediaFile.MediaID != "" {
			var episode database.Episode
			if err := db.Preload("Season").Preload("Season.TVShow").
				Where("id = ?", mediaFile.MediaID).First(&episode).Error; err == nil {
				response["metadata"] = gin.H{
					"type":           "episode",
					"episode_id":     episode.ID,
					"title":          episode.Title,
					"episode_number": episode.EpisodeNumber,
					"season": gin.H{
						"id":            episode.Season.ID,
						"season_number": episode.Season.SeasonNumber,
						"tv_show": gin.H{
							"id":    episode.Season.TVShow.ID,
							"title": episode.Season.TVShow.Title,
						},
					},
				}
			} else {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Episode metadata not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "File is not an episode",
			})
			return
		}
	case database.MediaTypeMovie:
		if mediaFile.MediaID != "" {
			var movie database.Movie
			if err := db.Where("id = ?", mediaFile.MediaID).First(&movie).Error; err == nil {
				response["metadata"] = gin.H{
					"type":     "movie",
					"movie_id": movie.ID,
					"title":    movie.Title,
					"release_date": movie.ReleaseDate,
					"runtime":  movie.Runtime,
				}
			} else {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Movie metadata not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "File is not a movie",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unknown media type",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
