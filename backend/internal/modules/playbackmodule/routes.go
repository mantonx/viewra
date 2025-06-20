package playbackmodule

import (
	"github.com/gin-gonic/gin"
)

// registerRoutes registers all playback module routes
func registerRoutes(router *gin.Engine, handler *APIHandler) {
	// Create playback API group
	playbackGroup := router.Group("/api/playback")
	{
		// Playback decision endpoint
		playbackGroup.POST("/decide", handler.HandlePlaybackDecision)

		// Session management endpoints
		playbackGroup.POST("/start", handler.HandleStartTranscode)
		playbackGroup.POST("/seek-ahead", handler.HandleSeekAhead)
		playbackGroup.GET("/session/:sessionId", handler.HandleGetSession)
		playbackGroup.DELETE("/session/:sessionId", handler.HandleStopTranscode)
		playbackGroup.GET("/sessions", handler.HandleListSessions)

		// Statistics endpoint
		playbackGroup.GET("/stats", handler.HandleGetStats)

		// Health check endpoint
		playbackGroup.GET("/health", handler.HandleHealthCheck)
		playbackGroup.HEAD("/health", handler.HandleHealthCheck)

		// Plugin management endpoints
		playbackGroup.POST("/plugins/refresh", handler.HandleRefreshPlugins)

		// Cleanup management endpoints
		playbackGroup.POST("/cleanup/run", handler.HandleManualCleanup)
		playbackGroup.GET("/cleanup/stats", handler.HandleCleanupStats)

		// DASH/HLS segment routes (specific patterns MUST come before catch-all)
		playbackGroup.GET("/stream/:sessionId/:segmentFile", handler.HandleDashSegmentSpecific)
		playbackGroup.HEAD("/stream/:sessionId/:segmentFile", handler.HandleDashSegmentSpecific)

		// DASH/HLS streaming endpoints
		playbackGroup.GET("/stream/:sessionId/manifest.mpd", handler.HandleDashManifest)
		playbackGroup.HEAD("/stream/:sessionId/manifest.mpd", handler.HandleDashManifest)
		playbackGroup.GET("/stream/:sessionId/playlist.m3u8", handler.HandleHlsPlaylist)
		playbackGroup.HEAD("/stream/:sessionId/playlist.m3u8", handler.HandleHlsPlaylist)
		playbackGroup.GET("/stream/:sessionId/segment/:segmentName", handler.HandleSegment)

		// Progressive streaming endpoint (catch-all MUST come last)
		playbackGroup.GET("/stream/:sessionId", handler.HandleStreamTranscode)
	}
}
