package playbackmodule

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all playback module routes
func RegisterRoutes(r *gin.Engine, handler *APIHandler) {
	api := r.Group("/api/playback")
	{
		// Decision endpoints
		api.POST("/decide", handler.HandlePlaybackDecision)

		// Session management
		api.POST("/start", handler.HandleStartTranscode)
		api.GET("/session/:sessionId", handler.HandleGetSession)
		api.DELETE("/session/:sessionId", handler.HandleStopTranscode)
		api.GET("/sessions", handler.HandleListSessions)

		// Seek-ahead functionality
		api.POST("/seek-ahead", handler.HandleSeekAhead)

		// Statistics and health
		api.GET("/stats", handler.HandleGetStats)
		api.GET("/health", handler.HandleHealthCheck)

		// Streaming endpoints
		api.GET("/stream/:sessionId", handler.HandleStreamTranscode)
		api.GET("/stream/:sessionId/manifest.mpd", handler.HandleDashManifest)
		api.GET("/stream/:sessionId/playlist.m3u8", handler.HandleHlsPlaylist)
		api.GET("/stream/:sessionId/segment/:segmentName", handler.HandleSegment)
		api.GET("/stream/:sessionId/:segmentFile", handler.HandleDashSegmentSpecific)

		// Cleanup endpoints
		api.POST("/cleanup/run", handler.HandleManualCleanup)
		api.GET("/cleanup/stats", handler.HandleCleanupStats)

		// Plugin management
		api.POST("/plugins/refresh", handler.HandleRefreshPlugins)
	}
}
