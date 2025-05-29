// Package handlers provides HTTP handlers for the plugin system.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
)

var pluginManager plugins.Manager

// InitializePluginManager initializes the global plugin manager
func InitializePluginManager(manager plugins.Manager) {
	pluginManager = manager
}

// =============================================================================
// PLUGIN MANAGEMENT ENDPOINTS
// =============================================================================

// GetPlugins returns all available plugins
func GetPlugins(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}

	// Get plugins from manager (filesystem discovery)
	pluginsList := pluginManager.ListPlugins()
	
	// Get database connection to fetch status information
	db := database.GetDB()
	
	// Create enhanced plugin list with database status
	enhancedPlugins := make([]map[string]interface{}, 0, len(pluginsList))
	
	for _, plugin := range pluginsList {
		// Get database status for this plugin
		var dbPlugin database.Plugin
		err := db.Where("plugin_id = ?", plugin.ID).First(&dbPlugin).Error
		
		// Create enhanced plugin info
		pluginInfo := map[string]interface{}{
			"id":          plugin.ID,
			"name":        plugin.Name,
			"version":     plugin.Version,
			"type":        plugin.Type,
			"description": plugin.Description,
			"author":      plugin.Author,
			"binary_path": plugin.BinaryPath,
			"config_path": plugin.ConfigPath,
			"base_path":   plugin.BasePath,
			"running":     plugin.Running,
		}
		
		if err == nil {
			// Plugin found in database, use database status
			pluginInfo["enabled"] = dbPlugin.Status == "enabled"
			pluginInfo["status"] = dbPlugin.Status
			pluginInfo["installed_at"] = dbPlugin.InstalledAt
			pluginInfo["enabled_at"] = dbPlugin.EnabledAt
			pluginInfo["error_message"] = dbPlugin.ErrorMessage
		} else {
			// Plugin not in database, show as discovered but not registered
			pluginInfo["enabled"] = false
			pluginInfo["status"] = "discovered"
			pluginInfo["installed_at"] = nil
			pluginInfo["enabled_at"] = nil
			pluginInfo["error_message"] = ""
		}
		
		enhancedPlugins = append(enhancedPlugins, pluginInfo)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugins": enhancedPlugins,
		"count":   len(enhancedPlugins),
	})
}

// GetPlugin returns information about a specific plugin
func GetPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}

	pluginInfo, exists := pluginManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin": pluginInfo,
	})
}

// GetPluginHealth returns the health status of a plugin
func GetPluginHealth(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}

	pluginInstance, exists := pluginManager.GetPlugin(pluginID)
	if !exists || !pluginInstance.Running {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found or not running",
		})
		return
	}

	healthy := false
	var healthErr error

	if pluginInstance.PluginService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, healthErr = pluginInstance.PluginService.Health(ctx, &proto.HealthRequest{})
		healthy = healthErr == nil
	} else {
		healthErr = fmt.Errorf("plugin does not expose a core PluginService for health checks")
	}

	status := gin.H{
		"plugin_id":  pluginID,
		"running":    pluginInstance.Running,
		"healthy":    healthy,
		"checked_at": time.Now(),
	}
	if healthErr != nil {
		status["error"] = healthErr.Error()
	}
	c.JSON(http.StatusOK, status)
}

// =============================================================================
// PLUGIN CONFIGURATION ENDPOINTS
// =============================================================================

// EnablePlugin enables a plugin and loads it
func EnablePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get database connection
	db := database.GetDB()

	// Check if plugin exists in manager
	plugin, exists := pluginManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}

	// Register plugin in database if not already registered
	var dbPlugin database.Plugin
	err := db.Where("plugin_id = ?", pluginID).First(&dbPlugin).Error
	if err != nil {
		// Plugin not in database, create it
		dbPlugin = database.Plugin{
			PluginID:    pluginID,
			Name:        plugin.Name,
			Version:     plugin.Version,
			Description: plugin.Description,
			Author:      plugin.Author,
			Type:        plugin.Type,
			Status:      "enabled",
			InstallPath: plugin.BasePath,
			InstalledAt: time.Now(),
			EnabledAt:   &[]time.Time{time.Now()}[0],
		}
		if err := db.Create(&dbPlugin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to register plugin in database",
				"details": err.Error(),
			})
			return
		}
	} else {
		// Plugin exists, update status to enabled
		now := time.Now()
		dbPlugin.Status = "enabled"
		dbPlugin.EnabledAt = &now
		if err := db.Save(&dbPlugin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update plugin status",
				"details": err.Error(),
			})
			return
		}
	}

	// Load the plugin if not already running
	if !plugin.Running {
		if err := pluginManager.LoadPlugin(ctx, pluginID); err != nil {
			// Update database status to error
			dbPlugin.Status = "error"
			dbPlugin.ErrorMessage = err.Error()
			db.Save(&dbPlugin)
			
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to load plugin",
				"details": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin enabled successfully",
		"plugin":  pluginID,
		"status":  "enabled",
	})
}

// DisablePlugin disables a plugin and unloads it
func DisablePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get database connection
	db := database.GetDB()

	// Update database status to disabled
	var dbPlugin database.Plugin
	err := db.Where("plugin_id = ?", pluginID).First(&dbPlugin).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found in database",
		})
		return
	}

	dbPlugin.Status = "disabled"
	dbPlugin.EnabledAt = nil
	if err := db.Save(&dbPlugin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update plugin status",
			"details": err.Error(),
		})
		return
	}

	// Unload the plugin if it's running
	plugin, exists := pluginManager.GetPlugin(pluginID)
	if exists && plugin.Running {
		if err := pluginManager.UnloadPlugin(ctx, pluginID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to unload plugin",
				"details": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin disabled successfully",
		"plugin":  pluginID,
		"status":  "disabled",
	})
}

// =============================================================================
// PLUGIN INSTALLATION ENDPOINTS
// =============================================================================

// InstallPlugin loads and starts a plugin (renamed from install for clarity)
func InstallPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt to load the plugin
	if err := pluginManager.LoadPlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to load plugin",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin loaded successfully",
		"plugin":  pluginID,
	})
}

// UninstallPlugin disables and removes a plugin
func UninstallPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Unload the plugin if it's running
	if err := pluginManager.UnloadPlugin(ctx, pluginID); err != nil {
		// Log error but attempt to continue if it's just "not running"
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to unload plugin",
			"details": err.Error(),
		})
		return
	}

	/* TODO: This was from the old system, actual removal of files needs careful consideration.
	// 2. Disable the plugin (remove from auto-load)
	if err := pluginManager.DisablePlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to disable plugin",
			"details": err.Error(),
		})
		return
	}
	*/

	// 3. TODO: Actual removal of plugin files from disk - this is a destructive operation
	//    And needs to be handled carefully. For now, we only unload/disable.
	// pluginPath := filepath.Join(pluginManager.PluginDir(), pluginID) // Assuming PluginDir() method exists
	// if err := os.RemoveAll(pluginPath); err != nil { ... }

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin unloaded. Manual deletion of files may be required.",
		"plugin":  pluginID,
	})
}

// =============================================================================
// PLUGIN EVENTS AND LOGS ENDPOINTS
// =============================================================================

// GetPluginEvents returns events for a specific plugin
func GetPluginEvents(c *gin.Context) {
	pluginID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	db := database.GetDB()
	
	// Get plugin database ID
	var dbPlugin database.Plugin
	if err := db.Where("plugin_id = ?", pluginID).First(&dbPlugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}
	
	// Get events
	var events []database.PluginEvent
	if err := db.Where("plugin_id = ?", dbPlugin.ID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve plugin events",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugin_id": pluginID,
		"events":    events,
		"count":     len(events),
		"limit":     limit,
		"offset":    offset,
	})
}

// GetAllPluginEvents returns events for all plugins
func GetAllPluginEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	db := database.GetDB()
	
	var events []database.PluginEvent
	if err := db.Preload("Plugin").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve plugin events",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
		"limit":  limit,
		"offset": offset,
	})
}

// =============================================================================
// PLUGIN DISCOVERY AND REGISTRY ENDPOINTS
// =============================================================================

// RefreshPlugins re-discovers plugins from the plugin directory
func RefreshPlugins(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	if err := pluginManager.DiscoverPlugins(); err != nil { // Removed context argument
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to refresh plugins",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugins refreshed successfully"})
}

// GetPluginManifest returns the manifest for a specific plugin
func GetPluginManifest(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin manager not initialized"})
		return
	}

	plugin, exists := pluginManager.GetPlugin(pluginID) // Changed to GetPlugin
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	// The concept of a separate "Manifest" object is gone.
	// Core plugin details are on the Plugin struct itself (plugin.ID, plugin.Name, etc.)
	// Configuration is in plugin.cue (plugin.ConfigPath points to it).
	// How to best expose this via API needs consideration. For now, return basic info.
	c.JSON(http.StatusOK, gin.H{
		"id":          plugin.ID,
		"name":        plugin.Name,
		"version":     plugin.Version,
		"type":        plugin.Type,
		"description": plugin.Description,
		"config_path": plugin.ConfigPath,
		// "schema": plugin.ConfigSchema, // ConfigSchema does not exist anymore
	})
}

// GetPluginAdminPages returns admin page configurations for a plugin
func GetPluginAdminPages(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	db := database.GetDB()
	
	var adminPages []database.PluginAdminPage
	if err := db.Preload("Plugin").
		Where("enabled = ?", true).
		Order("category, sort_order, title").
		Find(&adminPages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve admin pages",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"admin_pages": adminPages,
		"count":      len(adminPages),
	})
}

// GetPluginUIComponents returns UI components provided by plugins
func GetPluginUIComponents(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	db := database.GetDB()
	
	var components []database.PluginUIComponent
	if err := db.Preload("Plugin").
		Where("enabled = ?", true).
		Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve UI components",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"ui_components": components,
		"count":         len(components),
	})
}

// =============================================================================
// PLUGIN ROUTE DISPATCHING
// =============================================================================

// HandlePluginRoute handles dynamic routing to plugin endpoints
// URL format: /api/plugins/{plugin_id}/{plugin_route}
func HandlePluginRoute(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}

	// Parse the path: /plugin_id/route/path...
	path := strings.TrimPrefix(c.Param("path"), "/")
	pathParts := strings.SplitN(path, "/", 2)
	
	if len(pathParts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid plugin route format. Expected: /api/plugins/{plugin_id}/{route}",
		})
		return
	}
	
	pluginID := pathParts[0]
	pluginRoute := "/" + pathParts[1]
	
	// Get the plugin
	plugin, exists := pluginManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Plugin '%s' not found", pluginID),
		})
		return
	}
	
	if !plugin.Running {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": fmt.Sprintf("Plugin '%s' is not running", pluginID),
		})
		return
	}
	
	// Handle the route based on plugin implementation
	// For now, implement the MusicBrainz enricher routes
	if pluginID == "musicbrainz_enricher" {
		handleMusicBrainzRoute(c, pluginRoute, plugin)
		return
	}
	
	// Generic fallback for other plugins
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": fmt.Sprintf("Plugin '%s' route '%s' not implemented", pluginID, pluginRoute),
		"plugin_id": pluginID,
		"route": pluginRoute,
	})
}

// handleMusicBrainzRoute handles specific routes for the MusicBrainz enricher plugin
func handleMusicBrainzRoute(c *gin.Context, route string, plugin *plugins.Plugin) {
	switch {
	case route == "/config" && c.Request.Method == "GET":
		handleMusicBrainzConfig(c, plugin)
	case route == "/search" && c.Request.Method == "GET":
		handleMusicBrainzSearch(c, plugin)
	case route == "/enrichments" && c.Request.Method == "GET":
		handleMusicBrainzEnrichments(c)
	case strings.HasPrefix(route, "/enrich/") && c.Request.Method == "POST":
		handleMusicBrainzEnrich(c, route, plugin)
	default:
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("MusicBrainz route '%s' with method '%s' not found", route, c.Request.Method),
			"available_routes": []string{
				"GET /config - Get plugin configuration",
				"GET /search - Search MusicBrainz database",
				"GET /enrichments - Get existing enrichments",
				"POST /enrich/{mediaFileId} - Manually enrich a media file",
			},
		})
	}
}

// handleMusicBrainzConfig returns the current plugin configuration
func handleMusicBrainzConfig(c *gin.Context, plugin *plugins.Plugin) {
	config := gin.H{
		"plugin_id": plugin.ID,
		"name": plugin.Name,
		"version": plugin.Version,
		"description": plugin.Description,
		"running": plugin.Running,
		"configuration": gin.H{
			"enabled": true,
			"api_rate_limit": 0.8,
			"user_agent": "Viewra/2.0",
			"enable_artwork": true,
			"artwork_max_size": 1200,
			"artwork_quality": "front",
			"match_threshold": 0.85,
			"auto_enrich": true,
			"overwrite_existing": false,
			"cache_duration_hours": 168,
		},
		"endpoints": []gin.H{
			{
				"path": "/api/plugins/musicbrainz_enricher/config",
				"method": "GET",
				"description": "Get current plugin configuration",
			},
			{
				"path": "/api/plugins/musicbrainz_enricher/search",
				"method": "GET", 
				"description": "Search MusicBrainz for a track",
				"parameters": []gin.H{
					{"name": "title", "type": "string", "required": true, "description": "Song title"},
					{"name": "artist", "type": "string", "required": true, "description": "Artist name"},
					{"name": "album", "type": "string", "required": false, "description": "Album name"},
				},
			},
		},
	}
	
	c.JSON(http.StatusOK, config)
}

// handleMusicBrainzSearch performs a MusicBrainz search via the plugin
func handleMusicBrainzSearch(c *gin.Context, plugin *plugins.Plugin) {
	// Get query parameters
	title := c.Query("title")
	artist := c.Query("artist")
	album := c.Query("album")
	
	if title == "" || artist == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required parameters",
			"required": []string{"title", "artist"},
			"optional": []string{"album"},
			"example": "/api/plugins/musicbrainz_enricher/search?title=Bohemian%20Rhapsody&artist=Queen&album=A%20Night%20at%20the%20Opera",
		})
		return
	}
	
	// Use the dedicated SearchService
	if plugin.SearchService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin search service not available",
		})
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Build search query
	query := map[string]string{
		"title":  title,
		"artist": artist,
	}
	if album != "" {
		query["album"] = album
	}
	
	// Call the plugin's Search method
	searchResp, err := plugin.SearchService.Search(ctx, &proto.SearchRequest{
		Query:  query,
		Limit:  5,
		Offset: 0,
	})
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Search request failed",
			"details": err.Error(),
			"plugin_id": plugin.ID,
		})
		return
	}
	
	if !searchResp.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Plugin search failed",
			"details": searchResp.Error,
			"plugin_id": plugin.ID,
		})
		return
	}
	
	// Format the response
	response := gin.H{
		"query": gin.H{
			"title": title,
			"artist": artist,
			"album": album,
		},
		"plugin_info": gin.H{
			"plugin_id": plugin.ID,
			"name": plugin.Name,
			"version": plugin.Version,
			"running": plugin.Running,
		},
		"search_results": gin.H{
			"success": searchResp.Success,
			"total_count": searchResp.TotalCount,
			"has_more": searchResp.HasMore,
			"results": searchResp.Results,
		},
		"status": "success",
		"message": "MusicBrainz search completed successfully",
	}
	
	c.JSON(http.StatusOK, response)
}

// handleMusicBrainzEnrich handles the enrich route for the MusicBrainz enricher plugin
func handleMusicBrainzEnrich(c *gin.Context, route string, plugin *plugins.Plugin) {
	// Extract media file ID from route: /enrich/{mediaFileId}
	parts := strings.Split(route, "/")
	if len(parts) != 3 || parts[1] != "enrich" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid enrich route format. Expected: /enrich/{mediaFileId}",
		})
		return
	}
	
	mediaFileIDStr := parts[2]
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
			"details": err.Error(),
		})
		return
	}
	
	// Get the media file from database to extract metadata
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Database not available",
		})
		return
	}
	
	// Query media file with metadata
	var mediaFile struct {
		ID       uint   `json:"id"`
		Path     string `json:"path"`
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		Album    string `json:"album"`
	}
	
	err = db.Table("media_files").
		Select("media_files.id, media_files.path, music_metadata.title, music_metadata.artist, music_metadata.album").
		Joins("LEFT JOIN music_metadata ON media_files.id = music_metadata.media_file_id").
		Where("media_files.id = ?", mediaFileID).
		First(&mediaFile).Error
	
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
			"media_file_id": mediaFileID,
		})
		return
	}
	
	// Check if we have sufficient metadata
	if mediaFile.Title == "" || mediaFile.Artist == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Insufficient metadata for enrichment",
			"media_file_id": mediaFileID,
			"available_metadata": gin.H{
				"title": mediaFile.Title,
				"artist": mediaFile.Artist,
				"album": mediaFile.Album,
			},
		})
		return
	}
	
	// Use the working search API instead of broken gRPC
	if plugin.SearchService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin search service not available",
		})
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Build search query
	query := map[string]string{
		"title":  mediaFile.Title,
		"artist": mediaFile.Artist,
	}
	if mediaFile.Album != "" {
		query["album"] = mediaFile.Album
	}
	
	// Call the plugin's Search method
	searchResp, err := plugin.SearchService.Search(ctx, &proto.SearchRequest{
		Query:  query,
		Limit:  5,
		Offset: 0,
	})
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "MusicBrainz search failed",
			"details": err.Error(),
			"media_file_id": mediaFileID,
		})
		return
	}
	
	if !searchResp.Success || len(searchResp.Results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No MusicBrainz matches found",
			"media_file_id": mediaFileID,
			"query": query,
		})
		return
	}
	
	// Get the best match (first result, since they're sorted by score)
	bestMatch := searchResp.Results[0]
	
	// Check if match score is above threshold (0.85)
	if bestMatch.Score < 0.85 {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"error": "Best match score below threshold",
			"media_file_id": mediaFileID,
			"best_score": bestMatch.Score,
			"threshold": 0.85,
			"best_match": bestMatch,
		})
		return
	}
	
	// Define the enrichment struct (matching the plugin's model)
	type MusicBrainzEnrichment struct {
		ID                     uint      `gorm:"primaryKey"`
		MediaFileID            uint      `gorm:"not null;index"`
		MusicBrainzRecordingID string    `gorm:"size:36"`
		MusicBrainzArtistID    string    `gorm:"size:36"`
		MusicBrainzReleaseID   string    `gorm:"size:36"`
		EnrichedTitle          string    `gorm:"size:512"`
		EnrichedArtist         string    `gorm:"size:512"`
		EnrichedAlbum          string    `gorm:"size:512"`
		EnrichedGenre          string    `gorm:"size:255"`
		EnrichedYear           int
		MatchScore             float64
		EnrichedAt             time.Time `gorm:"autoCreateTime"`
		UpdatedAt              time.Time `gorm:"autoUpdateTime"`
	}
	
	// Create enrichment record
	enrichment := MusicBrainzEnrichment{
		MediaFileID:            uint(mediaFileID),
		MusicBrainzRecordingID: bestMatch.Id,
		EnrichedTitle:          bestMatch.Title,
		EnrichedArtist:         bestMatch.Artist,
		EnrichedAlbum:          bestMatch.Album,
		MatchScore:             bestMatch.Score,
		EnrichedAt:             time.Now(),
	}
	
	// Add metadata if available
	if artistID, ok := bestMatch.Metadata["artist_id"]; ok {
		enrichment.MusicBrainzArtistID = artistID
	}
	if releaseID, ok := bestMatch.Metadata["release_id"]; ok {
		enrichment.MusicBrainzReleaseID = releaseID
	}
	if tags, ok := bestMatch.Metadata["tags"]; ok {
		enrichment.EnrichedGenre = tags
	}
	if releaseDate, ok := bestMatch.Metadata["release_date"]; ok && len(releaseDate) >= 4 {
		if year, err := strconv.Atoi(releaseDate[:4]); err == nil {
			enrichment.EnrichedYear = year
		}
	}
	
	// Check if already enriched and remove old record
	db.Where("media_file_id = ?", mediaFileID).Delete(&MusicBrainzEnrichment{})
	
	// Save new enrichment
	if err := db.Create(&enrichment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save enrichment",
			"details": err.Error(),
			"media_file_id": mediaFileID,
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Enrichment created successfully",
		"media_file_id": mediaFileID,
		"enrichment": gin.H{
			"recording_id": enrichment.MusicBrainzRecordingID,
			"title": enrichment.EnrichedTitle,
			"artist": enrichment.EnrichedArtist,
			"album": enrichment.EnrichedAlbum,
			"genre": enrichment.EnrichedGenre,
			"year": enrichment.EnrichedYear,
			"match_score": enrichment.MatchScore,
		},
		"musicbrainz_match": bestMatch,
		"status": "success",
	})
}

// Add new route handler after the existing routes
func handleMusicBrainzEnrichments(c *gin.Context) {
	// Get database connection
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Database not available",
		})
		return
	}
	
	// Define the enrichment struct for querying
	type MusicBrainzEnrichment struct {
		ID                     uint      `json:"id" gorm:"primaryKey"`
		MediaFileID            uint      `json:"media_file_id" gorm:"not null;index"`
		MusicBrainzRecordingID string    `json:"recording_id" gorm:"size:36"`
		MusicBrainzArtistID    string    `json:"artist_id" gorm:"size:36"`
		MusicBrainzReleaseID   string    `json:"release_id" gorm:"size:36"`
		EnrichedTitle          string    `json:"title" gorm:"size:512"`
		EnrichedArtist         string    `json:"artist" gorm:"size:512"`
		EnrichedAlbum          string    `json:"album" gorm:"size:512"`
		EnrichedGenre          string    `json:"genre" gorm:"size:255"`
		EnrichedYear           int       `json:"year"`
		MatchScore             float64   `json:"match_score"`
		EnrichedAt             time.Time `json:"enriched_at" gorm:"autoCreateTime"`
		UpdatedAt              time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	}
	
	// Get query parameters
	mediaFileIDStr := c.Query("media_file_id")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	
	// Build query
	query := db.Model(&MusicBrainzEnrichment{})
	
	if mediaFileIDStr != "" {
		if mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32); err == nil {
			query = query.Where("media_file_id = ?", mediaFileID)
		}
	}
	
	// Get total count
	var total int64
	query.Count(&total)
	
	// Get enrichments with pagination
	var enrichments []MusicBrainzEnrichment
	err := query.Limit(limit).Offset(offset).Order("enriched_at DESC").Find(&enrichments).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve enrichments",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"enrichments": enrichments,
		"total": total,
		"limit": limit,
		"offset": offset,
		"has_more": total > int64(offset + limit),
	})
}
