// Package api - Playback decision handlers
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/types"
)

// DecidePlayback handles POST /api/v1/playback/decide
// It determines the best playback method for a media file based on device capabilities.
//
// Request body:
//   {
//     "media_path": "/path/to/file.mp4",
//     "device_profile": {
//       "supported_video_codecs": ["h264", "hevc"],
//       "supported_audio_codecs": ["aac", "mp3"],
//       "supported_containers": ["mp4", "mkv"],
//       "max_resolution": "1080p",
//       "max_bitrate": 10000000
//     }
//   }
//
// Response: PlaybackDecision with method (direct, remux, transcode) and parameters
func (h *Handler) DecidePlayback(c *gin.Context) {
	var req struct {
		MediaPath     string               `json:"media_path" binding:"required"`
		DeviceProfile *types.DeviceProfile `json:"device_profile" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	decision, err := h.playbackService.DecidePlayback(req.MediaPath, req.DeviceProfile)
	if err != nil {
		logger.Error("Failed to make playback decision", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, decision)
}

// GetMediaInfo handles GET /api/v1/playback/media-info
// It returns detailed technical information about a media file.
//
// Query parameters:
//   - path: The media file path
//
// Response: MediaInfo with video/audio streams, duration, bitrate, etc.
func (h *Handler) GetMediaInfo(c *gin.Context) {
	mediaPath := c.Query("path")
	if mediaPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Media path is required"})
		return
	}

	info, err := h.playbackService.GetMediaInfo(mediaPath)
	if err != nil {
		logger.Error("Failed to get media info", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetSupportedFormats handles GET /api/v1/playback/supported-formats
// It returns the media formats supported by a given device profile.
//
// Request body:
//   {
//     "supported_video_codecs": ["h264"],
//     "supported_audio_codecs": ["aac"],
//     "supported_containers": ["mp4"]
//   }
//
// Response: List of supported format combinations
func (h *Handler) GetSupportedFormats(c *gin.Context) {
	var deviceProfile types.DeviceProfile
	if err := c.ShouldBindJSON(&deviceProfile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device profile"})
		return
	}

	formats := h.playbackService.GetSupportedFormats(&deviceProfile)
	c.JSON(http.StatusOK, gin.H{
		"formats": formats,
		"count":   len(formats),
	})
}

// GetPlaybackCompatibility handles POST /api/v1/playback/compatibility
// It checks playback compatibility for multiple media files at once.
//
// Request body:
//   {
//     "media_file_ids": ["file1", "file2", "file3"],
//     "device_profile": { ... }
//   }
//
// Response:
//   {
//     "compatibility": {
//       "file1": {"method": "direct", "reason": "...", "can_direct_play": true},
//       "file2": {"method": "transcode", "reason": "...", "can_direct_play": false}
//     }
//   }
func (h *Handler) GetPlaybackCompatibility(c *gin.Context) {
	var req struct {
		MediaFileIds  []string             `json:"media_file_ids"`
		DeviceProfile *types.DeviceProfile `json:"device_profile"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Use a default device profile if none provided
	deviceProfile := req.DeviceProfile
	if deviceProfile == nil {
		deviceProfile = &types.DeviceProfile{
			SupportedVideoCodecs: []string{"h264"},
			SupportedAudioCodecs: []string{"aac", "mp3"},
			SupportedContainers:  []string{"mp4", "webm"},
		}
	}

	results := make(map[string]interface{})

	for _, mediaFileId := range req.MediaFileIds {
		// Get media file from media service
		ctx := c.Request.Context()
		mediaFile, err := h.mediaService.GetFile(ctx, mediaFileId)
		if err != nil {
			results[mediaFileId] = gin.H{
				"error": "Media file not found",
			}
			continue
		}

		// Get playback decision
		decision, err := h.playbackService.DecidePlayback(mediaFile.Path, deviceProfile)
		if err != nil {
			results[mediaFileId] = gin.H{
				"error": "Failed to determine compatibility",
			}
			continue
		}

		// Use the method from the decision
		canDirectPlay := decision.Method == types.PlaybackMethodDirect

		results[mediaFileId] = gin.H{
			"method":          string(decision.Method), // Convert to string for JSON
			"reason":          decision.Reason,
			"can_direct_play": canDirectPlay,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"compatibility": results,
	})
}