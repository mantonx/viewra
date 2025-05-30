// Package server provides HTTP server functionality for the Viewra application.
// This file contains all API route definitions organized by functionality.
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/server/handlers"
)

// setupRoutesWithEventHandlers configures routes with event handlers
func setupRoutesWithEventHandlers(r *gin.Engine) {
	// Static plugin assets - Using consistent path from GetPluginDirectory
	pluginsPath := GetPluginDirectory()
	r.Static("/plugins", pluginsPath)
	
	// API v1 routes group
	api := r.Group("/api")
	{
		setupHealthRoutes(api)
		
		api.POST("/dev/load-test-music", handlers.LoadTestMusicData)
		apiroutes.Register(api.BasePath()+"/dev/load-test-music", "POST", "Load test music data (development only).")
		
		// Plugin routes - handle all /api/plugins/* requests
		plugins := api.Group("/plugins")
		{
			plugins.Any("/*path", handlers.HandlePluginRoute)
		}
		
		// Event system routes
		// Ensure systemEventBus is checked before using, or handlers are robust to nil
		if systemEventBus != nil {
			eventsHandler := handlers.NewEventsHandler(systemEventBus)
			eventsGroup := api.Group("/events")
			{
				eventsGroup.GET("/", eventsHandler.GetEvents)
				apiroutes.Register(eventsGroup.BasePath()+"/", "GET", "List all recorded events.")

				eventsGroup.GET("/by-time", eventsHandler.GetEventsByTimeRange)
				apiroutes.Register(eventsGroup.BasePath()+"/by-time", "GET", "Get events within a specific time range.")

				eventsGroup.GET("/stats", eventsHandler.GetEventStats)
				apiroutes.Register(eventsGroup.BasePath()+"/stats", "GET", "Get statistics about recorded events.")

				eventsGroup.GET("/types", eventsHandler.GetEventTypes)
				apiroutes.Register(eventsGroup.BasePath()+"/types", "GET", "List all unique event types.")

				eventsGroup.GET("/stream", eventsHandler.EventStream)
				apiroutes.Register(eventsGroup.BasePath()+"/stream", "GET", "Stream events in real-time (SSE).")

				eventsGroup.POST("/", eventsHandler.PublishEvent)
				apiroutes.Register(eventsGroup.BasePath()+"/", "POST", "Publish a new event (for testing/dev).")

				eventsGroup.GET("/subscriptions", eventsHandler.GetSubscriptions)
				apiroutes.Register(eventsGroup.BasePath()+"/subscriptions", "GET", "List active event subscriptions.")

				eventsGroup.DELETE("/:id", eventsHandler.DeleteEvent)
				apiroutes.Register(eventsGroup.BasePath()+"/:id", "DELETE", "Delete a specific event by ID.")

				eventsGroup.DELETE("/", eventsHandler.ClearEvents)
				apiroutes.Register(eventsGroup.BasePath()+"/", "DELETE", "Clear all recorded events.")
			}
		}
			
		// Setup all routes with event-enabled handlers (now the standard)
		// Pass systemEventBus, handlers should be robust if it's nil or this block guarded by systemEventBus != nil
		setupMediaRoutesWithEvents(api, systemEventBus) // Renamed from setupMediaRoutesWithEvents
		setupUserRoutesWithEvents(api, systemEventBus)   // Renamed from setupUserRoutesWithEvents
		setupAdminRoutesWithEvents(api, systemEventBus)  // Renamed from setupAdminRoutesWithEvents
		
		// Call setupEventRoutes and setupScanRoutes here
		setupEventRoutes(api) // Ensures event routes are setup
		setupScanRoutes(api)  // Ensures scan routes are setup
		
		// Register module routes
		modulemanager.RegisterRoutes(r) // Should this be api? Check usage
	}

	// Add the root /api discovery endpoint directly to the main router `r`
	r.GET("/api", handlers.ApiRootHandler)
}

// =============================================================================
// HEALTH AND STATUS ROUTES
// =============================================================================

// setupHealthRoutes configures health check and status endpoints
func setupHealthRoutes(api *gin.RouterGroup) {
	api.GET("/health", handlers.HandleHealthCheck)
	apiroutes.Register(api.BasePath()+"/health", "GET", "System health check.")

	api.GET("/db-status", handlers.HandleDBStatus)
	apiroutes.Register(api.BasePath()+"/db-status", "GET", "Database connection status.")
}

// =============================================================================
// MEDIA ROUTES
// =============================================================================

// setupMediaRoutesWithEvents configures media-related endpoints with event support
func setupMediaRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	musicHandler := handlers.NewMusicHandler(eventBus) // Assumes eventBus can be nil if handlers support it
	mediaHandler := handlers.NewMediaHandler(eventBus) // Assumes eventBus can be nil if handlers support it
	media := api.Group("/media")
	{
		media.GET("/", mediaHandler.GetMedia)
		apiroutes.Register(media.BasePath()+"/", "GET", "List all media items.")

		media.GET("/:id", mediaHandler.GetMediaByID)
		apiroutes.Register(media.BasePath()+"/:id", "GET", "Get a specific media item by ID.")

		media.GET("/:id/stream", mediaHandler.StreamMedia)
		apiroutes.Register(media.BasePath()+"/:id/stream", "GET", "Stream a specific media file.")

		media.GET("/:id/artwork", handlers.GetArtwork) 
		apiroutes.Register(media.BasePath()+"/:id/artwork", "GET", "Get artwork for a media item.")

		media.GET("/:id/metadata", musicHandler.GetMusicMetadata)
		apiroutes.Register(media.BasePath()+"/:id/metadata", "GET", "Get metadata for a music item.")

		media.GET("/music", musicHandler.GetMusicFiles)
		apiroutes.Register(media.BasePath()+"/music", "GET", "List all music files.")

		playback := media.Group("/playback")
		{
			playback.POST("/start", musicHandler.RecordPlaybackStarted)
			apiroutes.Register(playback.BasePath()+"/start", "POST", "Record media playback started.")

			playback.POST("/end", musicHandler.RecordPlaybackFinished)
			apiroutes.Register(playback.BasePath()+"/end", "POST", "Record media playback finished.")

			playback.POST("/progress", musicHandler.RecordPlaybackProgress)
			apiroutes.Register(playback.BasePath()+"/progress", "POST", "Record media playback progress.")
		}
	}
}

// =============================================================================
// USER ROUTES
// =============================================================================

// setupUserRoutesWithEvents configures user management endpoints with event support
func setupUserRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	usersHandler := handlers.NewUsersHandler(eventBus) // Assumes eventBus can be nil if handlers support it
	users := api.Group("/users")
	{
		users.GET("/", usersHandler.GetUsers)
		apiroutes.Register(users.BasePath()+"/", "GET", "List all users.")

		users.POST("/", usersHandler.CreateUser)
		apiroutes.Register(users.BasePath()+"/", "POST", "Create a new user.")

		users.POST("/login", usersHandler.LoginUser)
		apiroutes.Register(users.BasePath()+"/login", "POST", "Login a user.")

		users.POST("/logout", usersHandler.LogoutUser)
		apiroutes.Register(users.BasePath()+"/logout", "POST", "Logout a user.")
	}
}

// =============================================================================
// ADMIN ROUTES
// =============================================================================

// setupAdminRoutesWithEvents configures administrative endpoints with event support
func setupAdminRoutesWithEvents(api *gin.RouterGroup, eventBus events.EventBus) {
	adminHandler := handlers.NewAdminHandler(eventBus) // Assumes eventBus can be nil if handlers support it
	admin := api.Group("/admin") // All admin routes are under /api/admin
	{
		libraries := admin.Group("/media-libraries")
		{
			libraries.GET("/", adminHandler.GetMediaLibraries)
			apiroutes.Register(libraries.BasePath()+"/", "GET", "List all media libraries.")
			libraries.POST("/", adminHandler.CreateMediaLibrary)
			apiroutes.Register(libraries.BasePath()+"/", "POST", "Create a new media library.")
			libraries.DELETE("/:id", adminHandler.DeleteMediaLibrary)
			apiroutes.Register(libraries.BasePath()+"/:id", "DELETE", "Delete a media library.")
			libraries.GET("/:id/stats", adminHandler.GetLibraryStats)
			apiroutes.Register(libraries.BasePath()+"/:id/stats", "GET", "Get statistics for a media library.")
			libraries.GET("/:id/files", adminHandler.GetMediaFiles)
			apiroutes.Register(libraries.BasePath()+"/:id/files", "GET", "List files in a media library.")
		}

		scanner := admin.Group("/scanner")
		{
			scanner.GET("/stats", handlers.GetScannerStats)
			apiroutes.Register(scanner.BasePath()+"/stats", "GET", "Get scanner statistics.")
			scanner.GET("/library-stats", handlers.GetAllLibraryStats)
			apiroutes.Register(scanner.BasePath()+"/library-stats", "GET", "Get statistics for all libraries.")
			scanner.GET("/status", handlers.GetScannerStatus)
			apiroutes.Register(scanner.BasePath()+"/status", "GET", "Get current scanner status.")
			scanner.GET("/current-jobs", handlers.GetCurrentJobs)
			apiroutes.Register(scanner.BasePath()+"/current-jobs", "GET", "List current scanner jobs.")
			scanner.POST("/start/:id", handlers.StartLibraryScanByID)
			apiroutes.Register(scanner.BasePath()+"/start/:id", "POST", "Start scanning a media library by ID.")
			scanner.POST("/pause/:id", handlers.StopLibraryScan)
			apiroutes.Register(scanner.BasePath()+"/pause/:id", "POST", "Pause scanning a media library.")
			scanner.POST("/stop/:id", handlers.StopLibraryScan) // Note: Same handler for pause and stop
			apiroutes.Register(scanner.BasePath()+"/stop/:id", "POST", "Stop scanning a media library.")
			scanner.POST("/resume/:id", handlers.ResumeLibraryScan)
			apiroutes.Register(scanner.BasePath()+"/resume/:id", "POST", "Resume scanning a media library.")
			scanner.POST("/cleanup-orphaned", handlers.CleanupOrphanedJobs)
			apiroutes.Register(scanner.BasePath()+"/cleanup-orphaned", "POST", "Cleanup orphaned scanner jobs.")
			scanner.DELETE("/jobs/:id", handlers.DeleteScanJob)
			apiroutes.Register(scanner.BasePath()+"/jobs/:id", "DELETE", "Delete a scan job and all its discovered files/assets.")
			scanner.GET("/progress/:id", handlers.GetScanProgress) // GET /api/admin/scanner/progress/:id
			apiroutes.Register(scanner.BasePath()+"/progress/:id", "GET", "Get scan progress for a library.")
		}

		pluginsGR := admin.Group("/plugins")
		{
			pluginsGR.GET("/", handlers.GetPlugins)
			apiroutes.Register(pluginsGR.BasePath()+"/", "GET", "List all available plugins.")
			pluginsGR.GET("/:id", handlers.GetPlugin)
			apiroutes.Register(pluginsGR.BasePath()+"/:id", "GET", "Get details for a specific plugin.")
			pluginsGR.GET("/:id/health", handlers.GetPluginHealth)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/health", "GET", "Get health status of a plugin.")
			pluginsGR.GET("/:id/events", handlers.GetPluginEvents)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/events", "GET", "Get events related to a plugin.")
			pluginsGR.GET("/events", handlers.GetAllPluginEvents)
			apiroutes.Register(pluginsGR.BasePath()+"/events", "GET", "Get events for all plugins.")
			pluginsGR.POST("/refresh", handlers.RefreshPlugins)
			apiroutes.Register(pluginsGR.BasePath()+"/refresh", "POST", "Refresh the list of available plugins.")
			pluginsGR.GET("/:id/manifest", handlers.GetPluginManifest)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/manifest", "GET", "Get manifest for a plugin.")
			pluginsGR.GET("/admin-pages", handlers.GetPluginAdminPages)
			apiroutes.Register(pluginsGR.BasePath()+"/admin-pages", "GET", "List admin pages provided by plugins.")
			pluginsGR.GET("/ui-components", handlers.GetPluginUIComponents)
			apiroutes.Register(pluginsGR.BasePath()+"/ui-components", "GET", "List UI components provided by plugins.")
			pluginsGR.POST("/:id/enable", handlers.EnablePlugin)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/enable", "POST", "Enable a plugin.")
			pluginsGR.POST("/:id/disable", handlers.DisablePlugin)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/disable", "POST", "Disable a plugin.")
			pluginsGR.POST("/:id/install", handlers.InstallPlugin)
			apiroutes.Register(pluginsGR.BasePath()+"/:id/install", "POST", "Install a plugin.")
			pluginsGR.DELETE("/:id", handlers.UninstallPlugin)
			apiroutes.Register(pluginsGR.BasePath()+"/:id", "DELETE", "Uninstall a plugin.")
		}
	}
}

// =============================================================================
// EVENT ROUTES
// =============================================================================

// setupEventRoutes configures event system endpoints
// This function is now called directly in setupRoutesWithEventHandlers.
// The actual event routes are defined within setupRoutesWithEventHandlers when systemEventBus is available.
// This function could be removed if all logic is within setupRoutesWithEventHandlers or if it's meant for other event related routes.
// For now, keeping it as a placeholder or for future expansion.
func setupEventRoutes(api *gin.RouterGroup) {
	// The main event routes (list, stream, publish etc.) are already set up
	// in setupRoutesWithEventHandlers based on systemEventBus availability.
	// This function can be used for additional event-related routes if needed
	// or can be removed if all event routes are handled above.
	// Example:
	// api.GET("/events/summary", handlers.GetEventSummary)
	// apiroutes.Register(api.BasePath()+"/events/summary", "GET", "Get a summary of event activity.")
}

// =============================================================================
// SCAN ROUTES
// =============================================================================

// setupScanRoutes configures scan endpoints for directory-based scanning
// This function is now called directly in setupRoutesWithEventHandlers.
func setupScanRoutes(api *gin.RouterGroup) {
	scan := api.Group("/scan") // These are /api/scan routes
	{
		scan.POST("/start", handlers.StartDirectoryScan)
		apiroutes.Register(scan.BasePath()+"/start", "POST", "Start a new directory scan.")

		scan.GET("/:id/progress", handlers.GetScanProgress)
		apiroutes.Register(scan.BasePath()+"/:id/progress", "GET", "Get progress of a specific scan job.")

		scan.POST("/:id/stop", handlers.StopScan)
		apiroutes.Register(scan.BasePath()+"/:id/stop", "POST", "Stop a specific scan job.")

		scan.POST("/:id/resume", handlers.ResumeScan)
		apiroutes.Register(scan.BasePath()+"/:id/resume", "POST", "Resume a specific scan job.")

		scan.GET("/:id/results", handlers.GetScanResults)
		apiroutes.Register(scan.BasePath()+"/:id/results", "GET", "Get results of a completed scan job.")
	}
}
