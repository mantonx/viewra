// Package server provides HTTP server functionality for the Viewra application.
// This file contains all API route definitions organized by functionality.
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
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
		setupEventRoutes(api)
		
		// Development routes - these would be removed in production
		api.POST("/dev/load-test-music", handlers.LoadTestMusicData)
	}
}

// setupRoutesWithEventHandlers configures routes with event handlers
func setupRoutesWithEventHandlers(r *gin.Engine) {
	// Static plugin assets
	r.Static("/plugins", "./data/plugins")
	
	// API v1 routes group
	api := r.Group("/api")
	{
		setupHealthRoutes(api)
		
		// Development routes - these would be removed in production
		api.POST("/dev/load-test-music", handlers.LoadTestMusicData)
		
		// Setup routes with event handlers if event bus is available
		if systemEventBus != nil {
			// Event system routes
			eventsHandler := handlers.NewEventsHandler(systemEventBus)
			events := api.Group("/events")
			{
				events.GET("/", eventsHandler.GetEvents)
				events.GET("/by-time", eventsHandler.GetEventsByTimeRange)
				events.GET("/stats", eventsHandler.GetEventStats)
				events.GET("/types", eventsHandler.GetEventTypes)
				events.GET("/stream", eventsHandler.EventStream)
				events.POST("/", eventsHandler.PublishEvent)
				events.GET("/subscriptions", eventsHandler.GetSubscriptions)
				events.DELETE("/:id", eventsHandler.DeleteEvent)
				events.DELETE("/", eventsHandler.ClearEvents)
			}
			
			// Setup all routes with event-enabled handlers
			setupMediaRoutesWithEvents(api, systemEventBus)
			setupUserRoutesWithEvents(api, systemEventBus)
			setupAdminRoutesWithEvents(api, systemEventBus)
		} else {
			// Fallback to basic routes without events
			setupMediaRoutes(api)
			setupUserRoutes(api)
			setupAdminRoutes(api)
		}
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

// setupMediaRoutesWithEvents configures media-related endpoints with event support
func setupMediaRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	// Create music handler with event bus
	musicHandler := handlers.NewMusicHandler(eventBus)
	
	// Create media handler with event bus  
	mediaHandler := handlers.NewMediaHandler(eventBus)
	
	media := api.Group("/media")
	{
		media.GET("/", mediaHandler.GetMedia)
		media.POST("/", mediaHandler.UploadMedia)
		media.GET("/:id/stream", mediaHandler.StreamMedia)      // GET /api/media/:id/stream - Stream media file
		media.GET("/:id/artwork", handlers.GetArtwork)         // Keep original handler for artwork
		media.GET("/:id/metadata", musicHandler.GetMusicMetadata) // GET /api/media/:id/metadata
		media.GET("/music", musicHandler.GetMusicFiles)        // GET /api/media/music
		
		// Add playback tracking endpoints with events
		playback := media.Group("/playback")
		{
			playback.POST("/start", musicHandler.RecordPlaybackStarted)
			playback.POST("/end", musicHandler.RecordPlaybackFinished)
			playback.POST("/progress", musicHandler.RecordPlaybackProgress)
		}
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

// setupUserRoutesWithEvents configures user management endpoints with event support
func setupUserRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	// Create users handler with event bus
	usersHandler := handlers.NewUsersHandler(eventBus)
	
	users := api.Group("/users")
	{
		users.GET("/", usersHandler.GetUsers)
		users.POST("/", usersHandler.CreateUser)
		
		// Add authentication endpoints with events
		users.POST("/login", usersHandler.LoginUser)
		users.POST("/logout", usersHandler.LogoutUser)
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
		}
	}
}

// setupAdminRoutesWithEvents configures administrative endpoints with event support
func setupAdminRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	// Create admin handler with event bus
	adminHandler := handlers.NewAdminHandler(eventBus)
	
	admin := api.Group("/admin")
	{
		// Media library management routes with events
		libraries := admin.Group("/media-libraries")
		{
			libraries.GET("/", adminHandler.GetMediaLibraries)
			libraries.POST("/", adminHandler.CreateMediaLibrary)
			libraries.DELETE("/:id", adminHandler.DeleteMediaLibrary)
			libraries.GET("/:id/stats", adminHandler.GetLibraryStats)
			libraries.GET("/:id/files", adminHandler.GetMediaFiles)
		}
		
		// Scanner routes (without events for now - keep original handlers)
		scanner := admin.Group("/scanner")
		{
			scanner.GET("/stats", handlers.GetScannerStats)                // GET /api/admin/scanner/stats
			scanner.GET("/status", handlers.GetScannerStatus)              // GET /api/admin/scanner/status  
			scanner.POST("/start/:id", handlers.StartLibraryScanByID)      // POST /api/admin/scanner/start/:id
			scanner.POST("/pause/:id", handlers.StopLibraryScan)           // POST /api/admin/scanner/pause/:id
			scanner.POST("/stop/:id", handlers.StopLibraryScan)            // POST /api/admin/scanner/stop/:id (for backward compatibility)
			scanner.POST("/resume/:id", handlers.ResumeLibraryScan)        // POST /api/admin/scanner/resume/:id
		}
	}
}

// =============================================================================
// EVENT ROUTES
// =============================================================================

// setupEventRoutes configures event system endpoints
func setupEventRoutes(api *gin.RouterGroup) {
	// Basic event routes will be set up by setupRoutesWithEventHandlers
	// This function is kept for compatibility
}
