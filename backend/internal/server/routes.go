// Package server provides HTTP server functionality for the Viewra application.
// This file contains all API route definitions organized by functionality.
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/server/handlers"
)

// setupRoutes configures all API routes for the Viewra application.
// It sets up versioned API endpoints organized by functionality:
// - Health and status endpoints for monitoring
// - Media management endpoints
// - User management endpoints
// - Admin endpoints for system configuration
func setupRoutes(r *gin.Engine) {
	// API v1 routes group
	api := r.Group("/api")
	{
		setupHealthRoutes(api)
		setupMediaRoutes(api)
		setupUserRoutes(api)
		setupAdminRoutes(api)
		
		// Development routes - these would be removed in production
		api.POST("/dev/load-test-music", handlers.LoadTestMusicData)
	}
}

// =============================================================================
// HEALTH AND STATUS ROUTES
// =============================================================================

// setupHealthRoutes configures health check and status endpoints
func setupHealthRoutes(api *gin.RouterGroup) {
	api.GET("/health", handlers.HandleHealthCheck)
	api.GET("/hello", handlers.HandleHello)
	api.GET("/db-status", handlers.HandleDBStatus)
}

// =============================================================================
// MEDIA ROUTES
// =============================================================================

// setupMediaRoutes configures media-related endpoints
func setupMediaRoutes(api *gin.RouterGroup) {
	media := api.Group("/media")
	{
		media.GET("/", handlers.GetMedia)
		media.POST("/", handlers.UploadMedia)
		media.GET("/:id/stream", handlers.StreamMedia)      // GET /api/media/:id/stream - Stream media file
		media.GET("/:id/artwork", handlers.GetArtwork)     // GET /api/media/:id/artwork
		media.GET("/:id/metadata", handlers.GetMusicMetadata) // GET /api/media/:id/metadata
		media.GET("/music", handlers.GetMusicFiles)        // GET /api/media/music
	}
}

// =============================================================================
// USER ROUTES
// =============================================================================

// setupUserRoutes configures user management endpoints
func setupUserRoutes(api *gin.RouterGroup) {
	users := api.Group("/users")
	{
		users.GET("/", handlers.GetUsers)
		users.POST("/", handlers.CreateUser)
	}
}

// =============================================================================
// ADMIN ROUTES
// =============================================================================

// setupAdminRoutes configures administrative endpoints
func setupAdminRoutes(api *gin.RouterGroup) {
	admin := api.Group("/admin")
	{
		// Media library management routes
		libraries := admin.Group("/media-libraries")
		{
			libraries.GET("/", handlers.GetMediaLibraries)
			libraries.POST("/", handlers.CreateMediaLibrary)
			libraries.DELETE("/:id", handlers.DeleteMediaLibrary)
			libraries.GET("/:id/stats", handlers.GetLibraryStats)
			libraries.GET("/:id/files", handlers.GetMediaFiles)
		}
		
		// Scanner routes
		scanner := admin.Group("/scanner")
		{
			scanner.GET("/stats", handlers.GetScannerStats)                // GET /api/admin/scanner/stats
			scanner.GET("/status", handlers.GetScannerStatus)              // GET /api/admin/scanner/status  
			scanner.POST("/start/:id", handlers.StartLibraryScanByID)      // POST /api/admin/scanner/start/:id
			scanner.POST("/pause/:id", handlers.StopLibraryScan)           // POST /api/admin/scanner/pause/:id
			scanner.POST("/stop/:id", handlers.StopLibraryScan)            // POST /api/admin/scanner/stop/:id (for backward compatibility)
			scanner.POST("/resume/:id", handlers.ResumeLibraryScan)        // POST /api/admin/scanner/resume/:id
			scanner.POST("/cancel-all", handlers.CancelAllScans)           // POST /api/admin/scanner/cancel-all
			scanner.GET("/library-stats", handlers.GetAllLibraryStats)     // GET /api/admin/scanner/library-stats
			scanner.GET("/scans", handlers.GetAllScans)                    // GET /api/admin/scanner/scans
			scanner.GET("/scan/:id", handlers.GetScanStatus)               // GET /api/admin/scanner/scan/:id
		}
	}
}