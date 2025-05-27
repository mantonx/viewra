// Package handlers provides HTTP handlers for the plugin system.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

	pluginsList := pluginManager.ListPlugins()
	c.JSON(http.StatusOK, gin.H{
		"plugins": pluginsList,
		"count":   len(pluginsList),
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

// =============================================================================
// PLUGIN INSTALLATION ENDPOINTS
// =============================================================================

// InstallPlugin installs a new plugin (placeholder - actual install might be manual or via other means)
func InstallPlugin(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Plugin installation not implemented via API yet"})
	// Actual implementation would involve:
	// 1. Receiving plugin package (e.g., zip file)
	// 2. Unpacking to plugin directory
	// 3. Calling pluginManager.DiscoverPlugins()
	// 4. Potentially auto-enabling or prompting user
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
