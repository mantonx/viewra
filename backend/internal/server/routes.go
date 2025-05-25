// Package server provides HTTP server functionality for the Viewra application.
// This file contains all API route definitions organized by functionality.
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/server/handlers"
)

// setupRoutes configures all API routes for the Viewra application.
// It sets up versioned API endpoints organized by functionality:
// - Health and status endpoints for monitoring
// - Media management endpoints
// - User management endpoints
// - Admin endpoints for system configuration
func setupRoutes(r *gin.Engine) {
	// Static plugin assets
	r.Static("/plugins", "./data/plugins")
	
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
			
			// Performance configuration routes
			scanner.GET("/config", handlers.GetScanConfig)                 // GET /api/admin/scanner/config
			scanner.PUT("/config", handlers.UpdateScanConfig)              // PUT /api/admin/scanner/config
			scanner.GET("/performance", handlers.GetScanPerformanceStats)  // GET /api/admin/scanner/performance
		}
		
		// Plugin management routes
		plugins := admin.Group("/plugins")
		{
			plugins.GET("/", handlers.GetPlugins)                          // GET /api/admin/plugins/
			plugins.POST("/refresh", handlers.RefreshPlugins)              // POST /api/admin/plugins/refresh
			plugins.POST("/install", handlers.InstallPlugin)               // POST /api/admin/plugins/install
			plugins.GET("/admin-pages", handlers.GetPluginAdminPages)      // GET /api/admin/plugins/admin-pages
			plugins.GET("/ui-components", handlers.GetPluginUIComponents)  // GET /api/admin/plugins/ui-components
			plugins.GET("/events", handlers.GetAllPluginEvents)            // GET /api/admin/plugins/events
			
			// Individual plugin routes
			plugins.GET("/:id", handlers.GetPlugin)                        // GET /api/admin/plugins/:id
			plugins.POST("/:id/enable", handlers.EnablePlugin)             // POST /api/admin/plugins/:id/enable
			plugins.POST("/:id/disable", handlers.DisablePlugin)           // POST /api/admin/plugins/:id/disable
			plugins.DELETE("/:id", handlers.UninstallPlugin)               // DELETE /api/admin/plugins/:id
			plugins.GET("/:id/health", handlers.GetPluginHealth)           // GET /api/admin/plugins/:id/health
			plugins.GET("/:id/config", handlers.GetPluginConfig)           // GET /api/admin/plugins/:id/config
			plugins.PUT("/:id/config", handlers.UpdatePluginConfig)        // PUT /api/admin/plugins/:id/config
			plugins.GET("/:id/manifest", handlers.GetPluginManifest)       // GET /api/admin/plugins/:id/manifest
			plugins.GET("/:id/events", handlers.GetPluginEvents)           // GET /api/admin/plugins/:id/events
		}
	}
}