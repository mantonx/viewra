// Music handler with event support
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
)

// MusicHandler handles music-related API endpoints
type MusicHandler struct {
	eventBus events.EventBus
}

// NewMusicHandler creates a new music handler with event bus
func NewMusicHandler(eventBus events.EventBus) *MusicHandler {
	return &MusicHandler{
		eventBus: eventBus,
	}
}

// GetMusicMetadata retrieves music metadata for a media file using new schema
func (h *MusicHandler) GetMusicMetadata(c *gin.Context) {
	idParam := c.Param("id")
	
	db := database.GetDB()
	
	// Find the media file
	var mediaFile database.MediaFile
	if err := db.Where("id = ?", idParam).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	// Check if it's a music file and get associated track
	if mediaFile.MediaType != database.MediaTypeTrack {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File is not a music track",
		})
		return
	}
	
	// Get the track with artist and album info
	var track database.Track
	if err := db.Preload("Artist").Preload("Album").Preload("Album.Artist").
		Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Track metadata not found",
		})
		return
	}
	
	// Return metadata in new format
	c.JSON(http.StatusOK, gin.H{
		"id":           track.ID,
		"title":        track.Title,
		"artist":       track.Artist.Name,
		"album":        track.Album.Title,
		"album_artist": track.Album.Artist.Name,
		"track_number": track.TrackNumber,
		"duration":     track.Duration,
		"lyrics":       track.Lyrics,
		"media_file":   mediaFile,
	})
}

// GetMusicFiles retrieves all music files with their metadata using new schema
func (h *MusicHandler) GetMusicFiles(c *gin.Context) {
	// Parse query parameters for pagination
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

	// Find all music libraries dynamically
	var musicLibraries []database.MediaLibrary
	err = db.Where("type = ?", "music").Find(&musicLibraries).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to find music libraries",
			"details": err.Error(),
		})
		return
	}

	if len(musicLibraries) == 0 {
		// No music libraries found
		c.JSON(http.StatusOK, gin.H{
			"music_files": []interface{}{},
			"count":       0,
			"total":       0,
			"limit":       limit,
			"offset":      offset,
		})
		return
	}

	// Extract library IDs
	var libraryIDs []uint32
	for _, lib := range musicLibraries {
		libraryIDs = append(libraryIDs, lib.ID)
	}

	// Query to fetch MediaFiles that are music tracks
	var mediaFiles []database.MediaFile
	err = db.Where("library_id IN ? AND media_type = ?", libraryIDs, database.MediaTypeTrack).
		Limit(limit).
		Offset(offset).
		Find(&mediaFiles).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve music files",
			"details": err.Error(),
		})
		return
	}
	
	// Get total count for all music libraries
	var total int64
	db.Model(&database.MediaFile{}).
		Where("library_id IN ? AND media_type = ?", libraryIDs, database.MediaTypeTrack).
		Count(&total)

	// Build response with track metadata
	var musicFilesWithMetadata []interface{}
	for _, mediaFile := range mediaFiles {
		// Get associated track info
		var track database.Track
		trackErr := db.Preload("Artist").Preload("Album").Preload("Album.Artist").
			Where("id = ?", mediaFile.MediaID).First(&track).Error
		
		fileData := map[string]interface{}{
			"id":           mediaFile.ID,
			"media_id":     mediaFile.MediaID,
			"media_type":   mediaFile.MediaType,
			"library_id":   mediaFile.LibraryID,
			"scan_job_id":  mediaFile.ScanJobID,
			"path":         mediaFile.Path,
			"container":    mediaFile.Container,
			"video_codec":  mediaFile.VideoCodec,
			"audio_codec":  mediaFile.AudioCodec,
			"channels":     mediaFile.Channels,
			"sample_rate":  mediaFile.SampleRate,
			"resolution":   mediaFile.Resolution,
			"duration":     mediaFile.Duration,
			"size_bytes":   mediaFile.SizeBytes,
			"bitrate_kbps": mediaFile.BitrateKbps,
			"language":     mediaFile.Language,
			"hash":         mediaFile.Hash,
			"version_name": mediaFile.VersionName,
			"last_seen":    mediaFile.LastSeen,
			"created_at":   mediaFile.CreatedAt,
			"updated_at":   mediaFile.UpdatedAt,
		}
		
		if trackErr == nil {
			// Include track metadata
			fileData["track"] = map[string]interface{}{
				"id":           track.ID,
				"title":        track.Title,
				"artist":       track.Artist.Name,
				"album":        track.Album.Title,
				"album_artist": track.Album.Artist.Name,
				"track_number": track.TrackNumber,
				"duration":     track.Duration,
				"lyrics":       track.Lyrics,
			}
		} else {
			// File exists but no track metadata yet
			fileData["track"] = nil
		}
		
		musicFilesWithMetadata = append(musicFilesWithMetadata, fileData)
	}

	// Create response
	response := gin.H{
		"music_files": musicFilesWithMetadata,
		"count":       len(musicFilesWithMetadata),
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	}

	// Emit event if there are music files
	if len(musicFilesWithMetadata) > 0 && h.eventBus != nil {
		event := events.NewSystemEvent(
			"music.files.retrieved",
			"Music Files Retrieved",
			fmt.Sprintf("Retrieved %d music files", len(musicFilesWithMetadata)),
		)
		event.Data = map[string]interface{}{
			"count":          len(musicFilesWithMetadata),
			"total":          total,
			"library_count":  len(musicLibraries),
		}
		h.eventBus.PublishAsync(event)
	}

	c.JSON(http.StatusOK, response)
}

// RecordPlaybackStarted records when playback of a track begins
func (h *MusicHandler) RecordPlaybackStarted(c *gin.Context) {
	var req struct {
		MediaID uint   `json:"mediaId" binding:"required"`
		UserID  uint   `json:"userId,omitempty"`
		Title   string `json:"title,omitempty"`
		Artist  string `json:"artist,omitempty"`
		Album   string `json:"album,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Publish playback started event
	if h.eventBus != nil {
		playEvent := events.NewSystemEvent(
			events.EventInfo,
			"Music Playback Started",
			fmt.Sprintf("Playing: %s by %s", req.Title, req.Artist),
		)
		playEvent.Data = map[string]interface{}{
			"mediaId":   req.MediaID,
			"userId":    req.UserID,
			"timestamp": time.Now().Unix(),
			"title":     req.Title,
			"artist":    req.Artist,
			"album":     req.Album,
		}
		h.eventBus.PublishAsync(playEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Playback started recorded",
	})
}

// RecordPlaybackFinished records when playback of a track ends
func (h *MusicHandler) RecordPlaybackFinished(c *gin.Context) {
	var req struct {
		MediaID     uint    `json:"mediaId" binding:"required"`
		UserID      uint    `json:"userId,omitempty"`
		Title       string  `json:"title,omitempty"`
		Artist      string  `json:"artist,omitempty"`
		Duration    float64 `json:"duration,omitempty"`    // Total track duration in seconds
		Listened    float64 `json:"listened,omitempty"`    // How much was actually listened to
		Completed   bool    `json:"completed,omitempty"`   // Whether the track was played to completion
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Publish playback finished event
	if h.eventBus != nil {
		playEvent := events.NewSystemEvent(
			events.EventInfo,
			"Music Playback Finished",
			fmt.Sprintf("Finished: %s by %s (%.1f%% played)", 
				req.Title, req.Artist, (req.Listened/req.Duration)*100),
		)
		playEvent.Data = map[string]interface{}{
			"mediaId":   req.MediaID,
			"userId":    req.UserID,
			"timestamp": time.Now().Unix(),
			"title":     req.Title,
			"artist":    req.Artist,
			"duration":  req.Duration,
			"listened":  req.Listened,
			"completed": req.Completed,
		}
		h.eventBus.PublishAsync(playEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Playback finished recorded",
	})
}

// RecordPlaybackProgress records playback progress updates
func (h *MusicHandler) RecordPlaybackProgress(c *gin.Context) {
	var req struct {
		MediaID     uint    `json:"mediaId" binding:"required"`
		UserID      uint    `json:"userId,omitempty"`
		Title       string  `json:"title,omitempty"`
		Position    float64 `json:"position"`     // Current position in seconds
		Duration    float64 `json:"duration"`     // Total duration in seconds
		Percentage  float64 `json:"percentage"`   // Percentage played (0-100)
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Only publish progress events at certain intervals to avoid spam
	// (e.g., every 25%, or for significant milestones)
	shouldPublish := false
	milestone := ""
	
	switch {
	case req.Percentage >= 25 && req.Percentage < 30:
		shouldPublish = true
		milestone = "25% played"
	case req.Percentage >= 50 && req.Percentage < 55:
		shouldPublish = true
		milestone = "50% played"  
	case req.Percentage >= 75 && req.Percentage < 80:
		shouldPublish = true
		milestone = "75% played"
	}
	
	if shouldPublish && h.eventBus != nil {
		progressEvent := events.NewSystemEvent(
			events.EventInfo,
			"Music Playback Progress",
			fmt.Sprintf("%s: %s (%s)", req.Title, milestone, 
				formatDuration(req.Position)+"/"+formatDuration(req.Duration)),
		)
		progressEvent.Data = map[string]interface{}{
			"mediaId":    req.MediaID,
			"userId":     req.UserID,
			"timestamp":  time.Now().Unix(),
			"title":      req.Title,
			"position":   req.Position,
			"duration":   req.Duration,
			"percentage": req.Percentage,
			"milestone":  milestone,
		}
		h.eventBus.PublishAsync(progressEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Progress recorded",
	})
}

// Helper function to format duration in MM:SS format
func formatDuration(seconds float64) string {
	mins := int(seconds) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

// GetMusicMetadata function-based handler for backward compatibility
func GetMusicMetadata(c *gin.Context) {
	// Create a handler instance and delegate to it
	handler := NewMusicHandler(nil)
	handler.GetMusicMetadata(c)
}

// GetMusicFiles function-based handler for backward compatibility
func GetMusicFiles(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MusicHandler{}
	handler.GetMusicFiles(c)
}