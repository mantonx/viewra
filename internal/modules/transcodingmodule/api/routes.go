package api

import (
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
)

// RegisterRoutes registers all transcoding module API routes.
// Routes are organized by functionality and follow RESTful conventions.
//
// API Structure:
//
//	/api/v1/transcoding
//	├── /transcode      - Transcoding operations
//	├── /providers      - Provider management
//	├── /sessions       - Session management
//	└── /progress       - Progress tracking
//
//	/api/v1/content
//	├── /:hash/:file    - Serve content files
//	├── /:hash/info     - Get content metadata
//	└── /stats          - Content storage statistics
func RegisterRoutes(router *gin.Engine, handler *APIHandler, contentHandler *ContentAPIHandler) {
	// API v1 group
	v1 := router.Group("/api/v1/transcoding")
	{
		// Transcoding operations
		v1.POST("/transcode", handler.StartTranscode)
		v1.DELETE("/transcode/:sessionId", handler.StopTranscode)

		// Progress tracking
		v1.GET("/progress/:sessionId", handler.GetProgress)

		// Session management
		v1.GET("/sessions", handler.ListSessions)
		v1.GET("/sessions/:sessionId", handler.GetSession)

		// Provider management
		v1.GET("/providers", handler.ListProviders)
		v1.GET("/providers/:providerId", handler.GetProvider)
		v1.GET("/providers/:providerId/formats", handler.GetProviderFormats)

		// Pipeline status (for pipeline provider)
		v1.GET("/pipeline/status", handler.GetPipelineStatus)

		// Content migration
		v1.GET("/content/stats", handler.GetContentHashStats)
		v1.GET("/content/sessions-without-hash", handler.ListSessionsWithoutContentHash)
		v1.POST("/content/migrate/:sessionId", handler.MigrateSessionToContentHash)
		v1.POST("/content/cleanup", handler.CleanupOldSessions)

		// Resource management
		v1.GET("/resources", handler.GetResourceUsage)
	}

	// Content-addressable storage routes
	if contentHandler != nil {
		contentGroup := router.Group("/api/v1/content")
		{
			// Storage statistics (specific routes first)
			contentGroup.GET("/stats", contentHandler.GetContentStats)
			
			// List content by media ID
			contentGroup.GET("/by-media/:mediaId", contentHandler.ListContentByMediaID)
			
			// Single handler for all content requests - it will distinguish between info and file requests
			contentGroup.GET("/:hash/*filepath", contentHandler.HandleContentRequest)
			contentGroup.HEAD("/:hash/*filepath", contentHandler.HandleContentRequest)
		}
		// Import logger at top of file if needed
		logger := hclog.New(&hclog.LoggerOptions{Name: "transcoding-api"})
		logger.Info("Content routes registered successfully")
	} else {
		logger := hclog.New(&hclog.LoggerOptions{Name: "transcoding-api"})
		logger.Warn("Content handler is nil, content routes not registered")
	}
}
