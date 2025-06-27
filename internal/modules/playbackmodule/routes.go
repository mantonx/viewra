package playbackmodule

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all playback module routes
func RegisterRoutes(r *gin.Engine, handler *APIHandler, sessionHandler *SessionHandler) {
	api := r.Group("/api/playback")
	{
		// Decision endpoints
		api.POST("/decide", handler.HandlePlaybackDecision)

		// Transcoding session management
		api.POST("/start", handler.HandleStartTranscode)
		api.DELETE("/session/:sessionId", handler.HandleStopTranscode)
		api.GET("/session/:sessionId", handler.HandleGetSession)
		api.POST("/seek-ahead", handler.HandleSeekAhead)
		api.DELETE("/sessions/all", handler.HandleStopAllSessions)
		api.GET("/sessions", handler.HandleListSessions)

		// Media validation
		api.POST("/validate", handler.HandleValidateMedia)

		// Statistics and health
		api.GET("/stats", handler.HandleGetStats)
		api.GET("/health", handler.HandleHealthCheck)

		// Error recovery stats
		api.GET("/error-recovery/stats", handler.HandleErrorRecoveryStats)

		// Plugin management
		api.POST("/plugins/refresh", handler.HandleRefreshPlugins)

		// Diagnostics (development)
		RegisterDiagnosticRoutes(api, handler)

		// FFmpeg monitoring
		RegisterMonitoringRoutes(api, handler)
	}

	// Session-based content routes (temporary URLs before content hash is available)
	v1 := r.Group("/api/v1")
	{
		// Session-based routes serve files directly from session directories
		v1.GET("/sessions/:sessionId/*file", sessionHandler.ServeSessionContent)
	}

	// Content-addressable storage routes are handled by the transcoding module
	// The playback module only handles session-based serving
}
