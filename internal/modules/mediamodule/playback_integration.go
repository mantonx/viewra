package mediamodule

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// PlaybackIntegration handles intelligent video playback and transcoding
type PlaybackIntegration struct {
	db              *gorm.DB
	playbackService services.PlaybackService
}

// NewPlaybackIntegration creates a new playback integration service
func NewPlaybackIntegration(db *gorm.DB, playbackService services.PlaybackService) *PlaybackIntegration {
	return &PlaybackIntegration{
		db:              db,
		playbackService: playbackService,
	}
}

// HandleIntelligentStream handles intelligent video streaming with automatic transcoding decisions
func (pi *PlaybackIntegration) HandleIntelligentStream(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Get media file from database
	var mediaFile MediaFileInfo
	if err := pi.getMediaFileInfo(fileID, &mediaFile); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Create device profile from request
	deviceProfile := pi.createDeviceProfileFromRequest(c)

	// Make intelligent playback decision using the service interface
	decision, err := pi.playbackService.DecidePlayback(mediaFile.Path, deviceProfile)
	if err != nil {
		log.Printf("ERROR: Failed to make playback decision for file_id=%s: %v", fileID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze media for playback"})
		return
	}

	log.Printf("INFO: Playback decision made for file_id=%s, should_transcode=%v, reason=%s, client_ip=%s",
		fileID, decision.ShouldTranscode, decision.Reason, c.ClientIP())

	if !decision.ShouldTranscode {
		// Direct streaming - serve the file directly
		pi.serveDirectStream(c, &mediaFile)
		return
	}

	// Need transcoding - start intelligent transcoding session
	if transcodeParams, ok := decision.TranscodeParams.(*plugins.TranscodeRequest); ok {
		pi.handleTranscodingStream(c, &mediaFile, transcodeParams)
	} else {
		log.Printf("ERROR: Invalid transcode parameters type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid transcode parameters"})
	}
}

// HandleStreamWithDecision handles streaming with an explicit transcoding decision
func (pi *PlaybackIntegration) HandleStreamWithDecision(c *gin.Context) {
	fileID := c.Param("id")
	forceTranscode := c.Query("transcode") == "true"
	quality := c.DefaultQuery("quality", "720p")

	// Get media file
	var mediaFile MediaFileInfo
	if err := pi.getMediaFileInfo(fileID, &mediaFile); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	if forceTranscode {
		// Force transcoding with specified quality
		transcodeParams := &plugins.TranscodeRequest{
			InputPath:      mediaFile.Path,
			OutputPath:     "", // Will be set by transcoding manager
			Container:      "mp4",
			VideoCodec:     "h264",
			AudioCodec:     "aac",
			Quality:        50, // Default medium quality
			SpeedPriority:  plugins.SpeedPriorityBalanced,
			PreferHardware: false,
			// Store resolution and bitrate in provider settings
			ProviderSettings: []byte(fmt.Sprintf(`{"resolution":"%s","bitrate":"%dk"}`, quality, pi.getBitrateForQuality(quality))),
		}

		pi.handleTranscodingStream(c, &mediaFile, transcodeParams)
		return
	}

	// Use intelligent decision
	pi.HandleIntelligentStream(c)
}

// HandlePlaybackDecision exposes the playback decision API
func (pi *PlaybackIntegration) HandlePlaybackDecision(c *gin.Context) {
	fileID := c.Param("id")

	// Get media file
	var mediaFile MediaFileInfo
	if err := pi.getMediaFileInfo(fileID, &mediaFile); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Create device profile
	deviceProfile := pi.createDeviceProfileFromRequest(c)

	// Make decision using the service interface
	decision, err := pi.playbackService.DecidePlayback(mediaFile.Path, deviceProfile)
	if err != nil {
		log.Printf("ERROR: ❌ Failed to decide playback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to decide playback method",
		})
		return
	}

	// Enhanced response with media file info
	response := gin.H{
		"should_transcode": decision.ShouldTranscode,
		"reason":           decision.Reason,
		"direct_play_url":  decision.DirectPlayURL,
		"media_info": gin.H{
			"id":          mediaFile.ID,
			"container":   mediaFile.Container,
			"video_codec": mediaFile.VideoCodec,
			"audio_codec": mediaFile.AudioCodec,
			"resolution":  mediaFile.Resolution,
			"duration":    mediaFile.Duration,
			"size_bytes":  mediaFile.SizeBytes,
		},
	}

	if decision.TranscodeParams != nil {
		response["transcode_params"] = decision.TranscodeParams

		// For transcoding recommendations, we'll trust that the service can handle it
		// The service layer will handle availability checks internally
		if transcodeParams, ok := decision.TranscodeParams.(*plugins.TranscodeRequest); ok {
			log.Printf("DEBUG: Transcoding recommended with params: codec=%s, container=%s",
				transcodeParams.VideoCodec, transcodeParams.Container)
		} else {
			log.Printf("DEBUG: Transcoding recommended with unknown parameters type")
		}

		// Check if we can fall back to direct streaming for unsupported transcoding scenarios
		if pi.isShakaPlayerCompatible(mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec) {
			log.Printf("INFO: Format %s with codecs %s/%s is Shaka-compatible, transcoding will be attempted first",
				mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec)
			response["stream_url"] = fmt.Sprintf("/api/media/files/%s/stream?transcode=true", fileID)
			response["fallback_stream_url"] = fmt.Sprintf("/api/media/files/%s/stream", fileID)
		} else {
			log.Printf("INFO: Format %s with codecs %s/%s requires transcoding",
				mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec)
			response["stream_url"] = fmt.Sprintf("/api/media/files/%s/stream?transcode=true", fileID)
		}
	} else {
		response["stream_url"] = fmt.Sprintf("/api/media/files/%s/stream", fileID)
	}

	c.JSON(http.StatusOK, response)
}

// handleTranscodingStream starts a transcoding session and streams the result
func (pi *PlaybackIntegration) handleTranscodingStream(c *gin.Context, mediaFile *MediaFileInfo, transcodeParams *plugins.TranscodeRequest) {
	// Add telemetry tracking
	sessionStartTime := time.Now()
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Extract resolution from provider settings
	targetResolution := ""
	if transcodeParams.ProviderSettings != nil {
		var settings map[string]interface{}
		if err := json.Unmarshal(transcodeParams.ProviderSettings, &settings); err == nil {
			if res, ok := settings["resolution"].(string); ok {
				targetResolution = res
			}
		}
	}
	targetContainer := transcodeParams.Container

	log.Printf("🔍 [TELEMETRY] Session start: file_id=%s client_ip=%s user_agent=%s container=%s video_codec=%s audio_codec=%s target_resolution=%s target_container=%s",
		mediaFile.ID, clientIP, userAgent, mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec, targetResolution, targetContainer)

	// Start transcoding session using the service interface
	ctx := c.Request.Context()
	session, err := pi.playbackService.StartTranscode(ctx, transcodeParams)
	if err != nil {
		log.Printf("🔍 [TELEMETRY] Transcoding failed to start: file_id=%s error=%v duration=%v",
			mediaFile.ID, err, time.Since(sessionStartTime))
		log.Printf("WARN: Failed to start transcoding session for file_id=%s: %v", mediaFile.ID, err)

		// Check if the source format and codecs are Shaka Player compatible for direct streaming
		if pi.isShakaPlayerCompatible(mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec) {
			log.Printf("INFO: Source format %s with codecs %s/%s is Shaka-compatible, falling back to direct streaming",
				mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec)
			pi.serveDirectStream(c, mediaFile)
		} else {
			log.Printf("ERROR: Source format %s with codecs %s/%s is not Shaka-compatible and transcoding failed for file_id=%s",
				mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec, mediaFile.ID)
			c.JSON(http.StatusNotImplemented, gin.H{
				"error":       "Media format not supported for direct streaming",
				"container":   mediaFile.Container,
				"video_codec": mediaFile.VideoCodec,
				"audio_codec": mediaFile.AudioCodec,
				"reason":      "Transcoding required but not available",
				"suggestion":  "Please configure a transcoding service to play this content",
			})
		}
		return
	}

	// Use Provider field instead of Backend
	log.Printf("🔍 [TELEMETRY] Transcoding started: file_id=%s session_id=%s provider=%s setup_duration=%v target_container=%s",
		mediaFile.ID, session.ID, session.Provider, time.Since(sessionStartTime), targetContainer)

	targetCodec := transcodeParams.VideoCodec
	log.Printf("INFO: Transcoding session started for file_id=%s, session_id=%s, provider=%s, target_codec=%s, resolution=%s, container=%s",
		mediaFile.ID, session.ID, session.Provider, targetCodec, targetResolution, targetContainer)

	// ===== CRITICAL FIX: Handle DASH/HLS adaptive streaming differently =====
	if targetContainer == "dash" || targetContainer == "hls" {
		log.Printf("🎬 ADAPTIVE STREAMING: Returning session info for %s adaptive streaming instead of progressive stream", strings.ToUpper(targetContainer))

		// For DASH/HLS, return session information so frontend can construct manifest URLs
		manifestEndpoint := ""
		if targetContainer == "dash" {
			manifestEndpoint = fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", session.ID)
		} else {
			manifestEndpoint = fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", session.ID)
		}

		log.Printf("🔍 [TELEMETRY] Adaptive streaming session info returned: session_id=%s container=%s manifest_url=%s setup_duration=%v",
			session.ID, targetContainer, manifestEndpoint, time.Since(sessionStartTime))

		// Return adaptive streaming session information
		c.Header("Content-Type", "application/json")
		c.Header("X-Adaptive-Streaming", "true")
		c.Header("X-Transcode-Session-ID", session.ID)
		c.Header("X-Transcode-Provider", session.Provider)
		c.Header("X-Transcode-Container", targetContainer)
		c.Header("X-Manifest-URL", manifestEndpoint)

		c.JSON(http.StatusOK, gin.H{
			"adaptive_streaming": true,
			"session_id":         session.ID,
			"container":          targetContainer,
			"manifest_url":       manifestEndpoint,
			"provider":           session.Provider,
			"resolution":         targetResolution,
			"message":            fmt.Sprintf("Use manifest URL for %s adaptive streaming", strings.ToUpper(targetContainer)),
		})
		return
	}

	// ===== PROGRESSIVE STREAMING (MP4) =====
	log.Printf("📱 PROGRESSIVE STREAMING: Starting progressive stream for MP4 container")

	// Set progressive streaming headers with proper codec information
	// H.264 (avc1.42E01E = Baseline Level 3.0) + AAC-LC (mp4a.40.2)
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Add transcoding session headers
	c.Header("X-Transcode-Session-ID", session.ID)
	c.Header("X-Transcode-Provider", session.Provider)
	c.Header("X-Transcode-Quality", targetResolution)

	// Create streaming request
	streamReq := &plugins.TranscodeRequest{
		SessionID:     session.ID,
		InputPath:     mediaFile.Path,
		Container:     "mp4", // Progressive streaming uses MP4
		VideoCodec:    targetCodec,
		AudioCodec:    "aac",
		Quality:       70, // Default balanced quality - providers can adjust based on resolution
		SpeedPriority: plugins.SpeedPriorityBalanced,
	}

	// Set resolution if different from original
	if targetResolution != "" && targetResolution != "original" {
		// Parse resolution to VideoResolution struct
		switch targetResolution {
		case "480p":
			res := plugins.Resolution480p
			streamReq.Resolution = &res
		case "720p":
			res := plugins.Resolution720p
			streamReq.Resolution = &res
		case "1080p":
			res := plugins.Resolution1080p
			streamReq.Resolution = &res
		case "1440p":
			res := plugins.Resolution1440p
			streamReq.Resolution = &res
		case "2160p", "4k":
			res := plugins.Resolution4K
			streamReq.Resolution = &res
		}
	}

	// For progressive streaming (MP4), we need to redirect to the transcoding session
	// The service has already started the transcoding, so we redirect to the session's output
	log.Printf("INFO: Progressive streaming - redirecting to transcoding session output")
	
	// Check if session is ready for streaming
	sessionInfo, err := pi.playbackService.GetSession(session.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get session info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get transcoding session info",
			"session_id": session.ID,
			"reason":     err.Error(),
		})
		return
	}
	
	// For now, return session info and let the client handle the streaming
	// This is a cleaner approach that works with the service interface
	c.Header("Content-Type", "application/json")
	c.Header("X-Transcode-Session-ID", session.ID)
	c.Header("X-Transcode-Provider", sessionInfo.Provider)
	c.Header("X-Transcode-Status", string(sessionInfo.Status))
	
	c.JSON(http.StatusOK, gin.H{
		"session_id":     session.ID,
		"provider":       sessionInfo.Provider,
		"status":         string(sessionInfo.Status),
		"stream_url":     fmt.Sprintf("/api/playback/stream/%s", session.ID),
		"container":      targetContainer,
		"resolution":     targetResolution,
		"message":        "Transcoding session started - use stream_url for progressive streaming",
	})
}

// serveDirectStream serves the file directly without transcoding
func (pi *PlaybackIntegration) serveDirectStream(c *gin.Context, mediaFile *MediaFileInfo) {
	log.Printf("INFO: Serving direct stream file_id=%s, container=%s, codecs=%s/%s, size=%d",
		mediaFile.ID, mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec, mediaFile.SizeBytes)

	// Use enhanced Content-Type with codec information for better Shaka Player compatibility
	contentType := pi.getContentTypeWithCodecs(mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec)

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("X-Direct-Stream", "true")
	c.Header("X-Original-Container", mediaFile.Container)
	c.Header("X-Video-Codec", mediaFile.VideoCodec)
	c.Header("X-Audio-Codec", mediaFile.AudioCodec)

	c.File(mediaFile.Path)
}

// createDeviceProfileFromRequest creates a device profile from the HTTP request
func (pi *PlaybackIntegration) createDeviceProfileFromRequest(c *gin.Context) *types.DeviceProfile {
	userAgent := c.GetHeader("User-Agent")
	clientIP := c.ClientIP()

	// Detect capabilities based on User-Agent and Accept headers
	supportedCodecs := []string{"h264", "aac"}
	supportsHEVC := false
	supportsAV1 := false
	maxResolution := "1080p"
	maxBitrate := 6000

	// Enhanced detection based on User-Agent
	accept := c.GetHeader("Accept")
	if accept != "" {
		// Check for advanced codec support
		// This is simplified - in production you'd have more sophisticated detection
		if contains(userAgent, "Chrome") || contains(userAgent, "Firefox") {
			supportsAV1 = true
			maxBitrate = 8000
		}
		if contains(userAgent, "Safari") {
			supportsHEVC = true
		}
	}

	// Check for explicit quality preference
	preferredQuality := c.Query("quality")
	if preferredQuality != "" {
		maxResolution = preferredQuality
		maxBitrate = pi.getBitrateForQuality(preferredQuality)
	}

	return &types.DeviceProfile{
		UserAgent:       userAgent,
		SupportedCodecs: supportedCodecs,
		MaxResolution:   maxResolution,
		MaxBitrate:      maxBitrate,
		SupportsHEVC:    supportsHEVC,
		SupportsAV1:     supportsAV1,
		SupportsHDR:     false, // Detected separately if needed
		ClientIP:        clientIP,
	}
}

// Helper functions

func (pi *PlaybackIntegration) getMediaFileInfo(fileID string, mediaFile *MediaFileInfo) error {
	return pi.db.Raw(`
		SELECT 
			id,
			path,
			COALESCE(container, '') as container,
			COALESCE(video_codec, '') as video_codec,
			COALESCE(audio_codec, '') as audio_codec,
			COALESCE(resolution, '') as resolution,
			COALESCE(duration, 0) as duration,
			size_bytes
		FROM media_files 
		WHERE id = ?
	`, fileID).Scan(mediaFile).Error
}

func (pi *PlaybackIntegration) getBitrateForQuality(quality string) int {
	switch quality {
	case "480p":
		return 1500
	case "720p":
		return 3000
	case "1080p":
		return 6000
	case "1440p":
		return 10000
	case "2160p":
		return 20000
	default:
		return 3000
	}
}

func (pi *PlaybackIntegration) getContentType(container string) string {
	switch container {
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "mkv":
		return "video/x-matroska"
	case "avi":
		return "video/x-msvideo"
	case "mov":
		return "video/quicktime"
	default:
		return "video/mp4"
	}
}

// getContentTypeWithCodecs returns Content-Type for better browser/player compatibility
func (pi *PlaybackIntegration) getContentTypeWithCodecs(container, videoCodec, audioCodec string) string {
	switch container {
	case "mp4":
		return "video/mp4"
	case "mkv":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	default:
		return pi.getContentType(container)
	}
}

// isWebCompatibleFormat checks if a container format is compatible with web browsers
func (pi *PlaybackIntegration) isWebCompatibleFormat(container string) bool {
	switch container {
	case "mp4", "webm", "ogg":
		return true
	case "mkv":
		// MKV requires special handling - only compatible with specific codecs
		// We'll check codecs separately in the calling function
		return false
	case "avi", "mov", "wmv", "flv", "m4v", "3gp", "mpg", "mpeg", "ts", "mts", "m2ts":
		return false
	default:
		// Default to false for unknown formats to be safe
		return false
	}
}

// isShakaPlayerCompatible checks if a media file is compatible with Shaka Player
func (pi *PlaybackIntegration) isShakaPlayerCompatible(container, videoCodec, audioCodec string) bool {
	container = strings.ToLower(container)
	videoCodec = strings.ToLower(videoCodec)
	audioCodec = strings.ToLower(audioCodec)

	// MP4 with H.264+AAC is always compatible
	if container == "mp4" && (videoCodec == "h264" || videoCodec == "h.264") && audioCodec == "aac" {
		return true
	}

	// WebM with VP8/VP9 + Vorbis/Opus is compatible
	if container == "webm" {
		if (videoCodec == "vp8" || videoCodec == "vp9") && (audioCodec == "vorbis" || audioCodec == "opus") {
			return true
		}
	}

	// MKV with H.264+AAC can work with Shaka Player in modern browsers
	// But we need to be more conservative here
	if container == "mkv" && (videoCodec == "h264" || videoCodec == "h.264") && audioCodec == "aac" {
		return true
	}

	return false
}

// isWebCompatibleCodecs checks if the video and audio codecs are web browser compatible
func (pi *PlaybackIntegration) isWebCompatibleCodecs(videoCodec, audioCodec string) bool {
	// Normalize codec names to lowercase for comparison
	videoCodec = strings.ToLower(videoCodec)
	audioCodec = strings.ToLower(audioCodec)

	// Web-compatible video codecs
	webVideoCodecs := map[string]bool{
		"h264":   true,
		"h.264":  true,
		"avc1":   true,
		"vp8":    true,
		"vp9":    true,
		"av1":    true,
		"theora": true,
	}

	// Web-compatible audio codecs
	webAudioCodecs := map[string]bool{
		"aac":    true,
		"mp3":    true,
		"opus":   true,
		"vorbis": true,
		"pcm":    true,
	}

	// Check if both video and audio codecs are web-compatible
	videoOK := webVideoCodecs[videoCodec]
	audioOK := webAudioCodecs[audioCodec] || audioCodec == "" // Allow empty audio codec

	return videoOK && audioOK
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MediaFileInfo represents media file information for playback decisions
type MediaFileInfo struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Container  string `json:"container"`
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
	Resolution string `json:"resolution"`
	Duration   int    `json:"duration"`
	SizeBytes  int64  `json:"size_bytes"`
}

// HandleIntelligentStreamHead handles HEAD requests by determining what headers the corresponding GET request would return
func (pi *PlaybackIntegration) HandleIntelligentStreamHead(c *gin.Context) {
	fileID := c.Param("id")
	forceTranscode := c.Query("transcode") == "true"

	// Get media file
	var mediaFile MediaFileInfo
	if err := pi.getMediaFileInfo(fileID, &mediaFile); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.Status(http.StatusNotFound)
		} else {
			c.Status(http.StatusInternalServerError)
		}
		return
	}

	if forceTranscode {
		// For forced transcoding, return transcoding headers
		c.Header("Content-Type", "video/mp4")
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")
		c.Header("X-Stream-Available", "true")
		c.Status(200)
		return
	}

	// For intelligent streaming, make the same decision as HandleIntelligentStream would make
	deviceProfile := pi.createDeviceProfileFromRequest(c)
	decision, err := pi.playbackService.DecidePlayback(mediaFile.Path, deviceProfile)
	if err != nil {
		log.Printf("ERROR: Failed to make playback decision for HEAD request: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if decision.ShouldTranscode {
		// Would transcode - return transcoding headers
		c.Header("Content-Type", "video/mp4")
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")
		c.Header("X-Stream-Available", "true")
	} else {
		// Would direct stream - return direct stream headers with proper codec information
		contentType := pi.getContentTypeWithCodecs(mediaFile.Container, mediaFile.VideoCodec, mediaFile.AudioCodec)
		c.Header("Content-Type", contentType)
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("X-Direct-Stream", "true")
		c.Header("X-Original-Container", mediaFile.Container)
		c.Header("X-Video-Codec", mediaFile.VideoCodec)
		c.Header("X-Audio-Codec", mediaFile.AudioCodec)
		c.Header("X-Stream-Available", "true")

		// Set content length if known
		if mediaFile.SizeBytes > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", mediaFile.SizeBytes))
		}
	}

	c.Status(200)
}
