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
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

var pluginModule *pluginmodule.PluginModule

// InitializePluginManager initializes the global plugin module
func InitializePluginManager(module *pluginmodule.PluginModule) {
	pluginModule = module
}

// =============================================================================
// PLUGIN MANAGEMENT ENDPOINTS
// =============================================================================

// GetPlugins returns all available plugins
func GetPlugins(c *gin.Context) {
	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin module not initialized",
		})
		return
	}

	// Get all plugins and enhance with database information
	allPlugins := pluginModule.ListAllPlugins()
	enhancedPlugins := make([]map[string]interface{}, 0, len(allPlugins))

	// Get database connection to fetch status information
	db := database.GetDB()

	for _, pluginInfo := range allPlugins {
		// Use Name for core plugins, ID for external plugins as identifier
		var pluginIdentifier string
		if pluginInfo.IsCore {
			pluginIdentifier = pluginInfo.Name
		} else {
			pluginIdentifier = pluginInfo.ID
		}

		// Create enhanced plugin info
		pluginData := map[string]interface{}{
			"id":          pluginIdentifier,
			"name":        pluginInfo.Name,
			"version":     pluginInfo.Version,
			"type":        pluginInfo.Type,
			"description": pluginInfo.Description,
			"author":      "", // External plugins may have author info
			"binary_path": "", // External plugins may have binary path
			"config_path": "", // External plugins may have config path
			"base_path":   "", // External plugins may have base path
			"running":     pluginInfo.Enabled,
			"enabled":     pluginInfo.Enabled,
			"status":      "unknown",
			"is_core":     pluginInfo.IsCore,
			"category":    pluginInfo.Category,
		}

		// Check database status for core plugins
		if pluginInfo.IsCore {
			var dbPlugin database.Plugin
			err := db.Where("plugin_id = ? AND type = ?", pluginIdentifier, "core").First(&dbPlugin).Error

			if err == nil {
				// Plugin found in database, use database status
				pluginData["enabled"] = dbPlugin.Status == "enabled"
				pluginData["status"] = dbPlugin.Status
				pluginData["installed_at"] = dbPlugin.InstalledAt
				pluginData["enabled_at"] = dbPlugin.EnabledAt
				pluginData["error_message"] = dbPlugin.ErrorMessage
			} else {
				// Plugin not in database, show as discovered but not registered
				pluginData["enabled"] = true // Core plugins default to enabled
				pluginData["status"] = "enabled"
				pluginData["installed_at"] = nil
				pluginData["enabled_at"] = nil
				pluginData["error_message"] = ""
			}
		} else {
			// External plugin - get details from external manager
			if externalPlugin, exists := pluginModule.GetExternalPlugin(pluginIdentifier); exists {
				pluginData["binary_path"] = externalPlugin.Path
				pluginData["running"] = externalPlugin.Running
				pluginData["enabled"] = externalPlugin.Running
				pluginData["status"] = "discovered"
				if externalPlugin.Running {
					pluginData["status"] = "running"
				}
			}
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

	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin module not initialized",
		})
		return
	}

	// Try to find as core plugin first
	if corePlugin, exists := pluginModule.GetCorePlugin(pluginID); exists {
		pluginInfo := map[string]interface{}{
			"id":         corePlugin.GetName(),
			"name":       corePlugin.GetName(),
			"type":       corePlugin.GetPluginType(),
			"enabled":    corePlugin.IsEnabled(),
			"is_core":    true,
			"extensions": corePlugin.GetSupportedExtensions(),
		}

		c.JSON(http.StatusOK, gin.H{
			"plugin": pluginInfo,
		})
		return
	}

	// Try to find as external plugin
	if externalPlugin, exists := pluginModule.GetExternalPlugin(pluginID); exists {
		c.JSON(http.StatusOK, gin.H{
			"plugin": externalPlugin,
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": "Plugin not found",
	})
}

// GetPluginHealth returns the health status of a plugin
func GetPluginHealth(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin module not initialized",
		})
		return
	}

	// For core plugins, check if they're enabled and initialized
	if corePlugin, exists := pluginModule.GetCorePlugin(pluginID); exists {
		status := gin.H{
			"plugin_id":  pluginID,
			"running":    corePlugin.IsEnabled(),
			"healthy":    corePlugin.IsEnabled(),
			"checked_at": time.Now(),
			"is_core":    true,
		}
		c.JSON(http.StatusOK, status)
		return
	}

	// For external plugins, check if they're running
	if externalPlugin, exists := pluginModule.GetExternalPlugin(pluginID); exists {
		status := gin.H{
			"plugin_id":  pluginID,
			"running":    externalPlugin.Running,
			"healthy":    externalPlugin.Running, // Simple check for now
			"checked_at": time.Now(),
			"is_core":    false,
		}
		c.JSON(http.StatusOK, status)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": "Plugin not found or not running",
	})
}

// =============================================================================
// PLUGIN CONFIGURATION ENDPOINTS
// =============================================================================

// EnablePlugin enables a plugin and loads it
func EnablePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin module not initialized"})
		return
	}

	// Try enabling as core plugin first
	if err := pluginModule.EnableCorePlugin(pluginID); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Core plugin enabled successfully",
			"plugin_id": pluginID,
		})
		return
	}

	// Try loading as external plugin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pluginModule.LoadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to enable plugin: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "External plugin loaded successfully",
		"plugin_id": pluginID,
	})
}

// DisablePlugin disables a plugin
func DisablePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin module not initialized"})
		return
	}

	// Try disabling as core plugin first
	if err := pluginModule.DisableCorePlugin(pluginID); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Core plugin disabled successfully",
			"plugin_id": pluginID,
		})
		return
	}

	// Try unloading as external plugin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pluginModule.UnloadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to disable plugin: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "External plugin unloaded successfully",
		"plugin_id": pluginID,
	})
}

// InstallPlugin installs a new plugin
func InstallPlugin(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Plugin installation not yet implemented",
	})
}

// UninstallPlugin uninstalls a plugin
func UninstallPlugin(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Plugin uninstallation not yet implemented",
	})
}

// =============================================================================
// PLUGIN EVENT ENDPOINTS
// =============================================================================

// GetPluginEvents returns events for a specific plugin
func GetPluginEvents(c *gin.Context) {
	pluginID := c.Param("id")

	// Get query parameters for pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	db := database.GetDB()

	var events []database.PluginEvent
	var total int64

	// Count total events for this plugin
	db.Model(&database.PluginEvent{}).Where("plugin_id = ?", pluginID).Count(&total)

	// Get paginated events
	err := db.Where("plugin_id = ?", pluginID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&events).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve plugin events",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetAllPluginEvents returns events for all plugins
func GetAllPluginEvents(c *gin.Context) {
	// Get query parameters for pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	eventType := c.Query("event_type")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	db := database.GetDB()

	var events []database.PluginEvent
	var total int64

	// Build query
	query := db.Model(&database.PluginEvent{})
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	// Count total events
	query.Count(&total)

	// Get paginated events
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&events).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve plugin events",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// RefreshPlugins refreshes the plugin list
func RefreshPlugins(c *gin.Context) {
	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin module not initialized",
		})
		return
	}

	// Refresh external plugins
	if err := pluginModule.RefreshExternalPlugins(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to refresh external plugins: %v", err),
		})
		return
	}

	// Core plugins are auto-loaded on startup, but we could add refresh logic here if needed

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin list refreshed successfully",
	})
}

// GetPluginManifest returns the manifest for a plugin
func GetPluginManifest(c *gin.Context) {
	pluginID := c.Param("id")

	if pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin module not initialized",
		})
		return
	}

	// Check if it's a core plugin
	if corePlugin, exists := pluginModule.GetCorePlugin(pluginID); exists {
		manifest := gin.H{
			"id":          corePlugin.GetName(),
			"name":        corePlugin.GetName(),
			"version":     "1.0.0",
			"type":        corePlugin.GetPluginType(),
			"description": "Built-in core plugin",
			"is_core":     true,
			"extensions":  corePlugin.GetSupportedExtensions(),
		}

		c.JSON(http.StatusOK, gin.H{
			"manifest": manifest,
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": "Plugin manifest not found",
	})
}

// GetPluginAdminPages returns admin pages provided by plugins
func GetPluginAdminPages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"admin_pages": []interface{}{},
		"message":     "No admin pages available",
	})
}

// GetPluginUIComponents returns UI components provided by plugins
func GetPluginUIComponents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ui_components": []interface{}{},
		"message":       "No UI components available",
	})
}

// =============================================================================
// PLUGIN ROUTE PROXY
// =============================================================================

// HandlePluginRoute handles dynamic plugin routes
func HandlePluginRoute(c *gin.Context) {
	// Extract plugin path from the URL
	pluginPath := c.Param("path")

	// Remove leading slash if present
	pluginPath = strings.TrimPrefix(pluginPath, "/")

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":       "Plugin routes not yet implemented",
		"plugin_path": pluginPath,
		"method":      c.Request.Method,
	})
}
