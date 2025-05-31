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

	// Get all plugins and enhance with database information
	allPlugins := pluginManager.ListPlugins()
	enhancedPlugins := make([]map[string]interface{}, 0, len(allPlugins))
	
	// Get database connection to fetch status information
	db := database.GetDB()
	
	for _, pluginInfo := range allPlugins {
		// Use ID for external plugins, Name for core plugins as identifier
		var pluginIdentifier string
		if pluginInfo.IsCore {
			pluginIdentifier = pluginInfo.Name
		} else {
			pluginIdentifier = pluginInfo.ID
		}
		
		// Get the full plugin object for additional details
		plugin, exists := pluginManager.GetPlugin(pluginIdentifier)
		if !exists {
			// If plugin doesn't exist in manager, use info from PluginInfo
			pluginData := map[string]interface{}{
				"id":          pluginIdentifier,
				"name":        pluginInfo.Name,
				"version":     pluginInfo.Version,
				"type":        pluginInfo.Type,
				"description": pluginInfo.Description,
				"author":      "",
				"binary_path": "",
				"config_path": "",
				"base_path":   "",
				"running":     pluginInfo.Enabled,
				"enabled":     pluginInfo.Enabled,
				"status":      "unknown",
				"is_core":     pluginInfo.IsCore,
				"category":    pluginInfo.Category,
			}
			enhancedPlugins = append(enhancedPlugins, pluginData)
			continue
		}

		// Check database status
		var dbPlugin database.Plugin
		err := db.Where("plugin_id = ?", pluginIdentifier).First(&dbPlugin).Error
		
		// Create enhanced plugin info
		pluginData := map[string]interface{}{
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
			"is_core":     pluginInfo.IsCore,
			"category":    pluginInfo.Category,
		}
		
		if err == nil {
			// Plugin found in database, use database status
			pluginData["enabled"] = dbPlugin.Status == "enabled"
			pluginData["status"] = dbPlugin.Status
			pluginData["installed_at"] = dbPlugin.InstalledAt
			pluginData["enabled_at"] = dbPlugin.EnabledAt
			pluginData["error_message"] = dbPlugin.ErrorMessage
		} else {
			// Plugin not in database, show as discovered but not registered
			pluginData["enabled"] = false
			pluginData["status"] = "discovered"
			pluginData["installed_at"] = nil
			pluginData["enabled_at"] = nil
			pluginData["error_message"] = ""
		}
		
		enhancedPlugins = append(enhancedPlugins, pluginData)
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
	
	if len(pathParts) < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid plugin route format. Expected: /api/plugins/{plugin_id}/{route}",
		})
		return
	}
	
	pluginID := pathParts[0]
	pluginRoute := "/"
	if len(pathParts) > 1 {
		pluginRoute = "/" + pathParts[1]
	}
	
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
	
	// Generic plugin route handling - delegate to the plugin's HTTP service
	if plugin.PluginService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": fmt.Sprintf("Plugin '%s' does not expose HTTP endpoints", pluginID),
			"plugin_id": pluginID,
			"route": pluginRoute,
		})
		return
	}
	
	// TODO: Implement generic HTTP forwarding to plugin's HTTP service
	// This would involve converting the Gin request to a proto HTTP request
	// and forwarding it to the plugin's HTTP handler, then converting the response back
	
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Generic plugin HTTP forwarding not yet implemented",
		"message": "Plugin-specific routes should be handled by the plugin's own HTTP service",
		"plugin_id": pluginID,
		"route": pluginRoute,
		"suggestion": "Use the plugin's dedicated service endpoints or implement HTTP forwarding",
	})
}