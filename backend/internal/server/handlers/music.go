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

// GetMusicMetadata retrieves music metadata for a media file
func (h *MusicHandler) GetMusicMetadata(c *gin.Context) {
	// TODO: With new schema, music metadata would be retrieved through Artist/Album/Track relationships
	// For now, return a placeholder response
	c.JSON(http.StatusNotFound, gin.H{
		"error": "Music metadata retrieval not yet implemented for new schema",
		"note":  "Music metadata is now stored in Artist/Album/Track tables",
	})
}

// GetMusicFiles retrieves all music files with their metadata
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
			"music_files": []database.MediaFile{},
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

	// Query to fetch MediaFiles with their MusicMetadata preloaded from all music libraries
	var mediaFiles []database.MediaFile
	err = db.Preload("MusicMetadata").
		Where("library_id IN ?", libraryIDs).
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
	db.Model(&database.MediaFile{}).Where("library_id IN ?", libraryIDs).Count(&total)

	// Use all media files - even if some don't have metadata yet
	// This ensures the files appear in the UI while metadata is still being processed
	var musicFiles []database.MediaFile = mediaFiles

	c.JSON(http.StatusOK, gin.H{
		"music_files": musicFiles,
		"count":       len(musicFiles),
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
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

// Keep original function-based handlers for backward compatibility
// These will delegate to the struct-based handlers

// GetMusicMetadata function-based handler for backward compatibility
func GetMusicMetadata(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MusicHandler{}
	handler.GetMusicMetadata(c)
}

// GetMusicFiles function-based handler for backward compatibility
func GetMusicFiles(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MusicHandler{}
	handler.GetMusicFiles(c)
}