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
//	├── /health         - Health check
//	├── /transcode      - Transcoding operations
//	├── /providers      - Provider management
//	├── /sessions       - Session management
//	├── /stats          - Statistics and monitoring
//	└── /resources      - Resource usage
//
//	/api/v1/content
//	├── /:hash/*file    - Serve content files
//	├── /stats          - Content storage statistics
//	└── /by-media/:id   - List content by media ID
func RegisterRoutes(router *gin.Engine, handler *APIHandler, contentHandler *ContentAPIHandler) {
	// API v1 group
	v1 := router.Group("/api/v1/transcoding")
	{
		// Health check
		v1.GET("/health", handler.HealthCheck)

		// Transcoding operations
		v1.POST("/transcode", handler.StartTranscode)

		// Session management
		v1.GET("/sessions", handler.ListSessions)
		v1.GET("/sessions/:id", handler.GetSession)
		v1.GET("/sessions/:id/progress", handler.GetProgress)
		v1.DELETE("/sessions/:id", handler.StopTranscode)

		// Provider management
		v1.GET("/providers", handler.GetProviders)
		v1.GET("/providers/:id", handler.GetProvider)

		// Statistics and monitoring
		v1.GET("/stats", handler.GetStats)
		v1.GET("/pipeline/status", handler.GetPipelineStatus)
		v1.GET("/resources", handler.GetResourceUsage)

		// Content hash migration
		v1.GET("/content-hash/stats", handler.GetContentHashStats)
		v1.POST("/content-hash/migrate", handler.MigrateContentHashes)
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
		// Log successful registration
		logger := hclog.New(&hclog.LoggerOptions{Name: "transcoding-api"})
		logger.Info("Content routes registered successfully")
	} else {
		logger := hclog.New(&hclog.LoggerOptions{Name: "transcoding-api"})
		logger.Warn("Content handler is nil, content routes not registered")
	}
}