// Package handlers provides HTTP handlers for the plugin system.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
)

var pluginManager *plugins.Manager

// InitializePluginManager initializes the global plugin manager
func InitializePluginManager(manager *plugins.Manager) {
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

	plugins := pluginManager.ListPlugins()
	
	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
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
	
	info, exists := pluginManager.GetPluginInfo(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugin": info,
	})
}

// EnablePlugin enables a plugin
func EnablePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := pluginManager.EnablePlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to enable plugin",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin enabled successfully",
		"plugin":  pluginID,
	})
}

// DisablePlugin disables a plugin
func DisablePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := pluginManager.DisablePlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to disable plugin",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin disabled successfully",
		"plugin":  pluginID,
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
	
	plugin, exists := pluginManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not loaded",
		})
		return
	}
	
	healthy := plugin.Health() == nil
	
	c.JSON(http.StatusOK, gin.H{
		"plugin":  pluginID,
		"healthy": healthy,
		"status":  map[string]interface{}{
			"running": healthy,
			"checked_at": time.Now(),
		},
	})
}

// =============================================================================
// PLUGIN CONFIGURATION ENDPOINTS
// =============================================================================

// GetPluginConfig returns the configuration for a plugin
func GetPluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	info, exists := pluginManager.GetPluginInfo(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugin_id": pluginID,
		"config":    info.Config,
		"schema":    info.Manifest.ConfigSchema,
	})
}

// UpdatePluginConfig updates the configuration for a plugin
func UpdatePluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	var configRequest struct {
		Config map[string]interface{} `json:"config" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&configRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}
	
	// Get plugin info
	info, exists := pluginManager.GetPluginInfo(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}
	
	// Update configuration
	info.Config = configRequest.Config
	info.UpdatedAt = time.Now()
	
	// Update in database
	configData, err := json.Marshal(info.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to serialize configuration",
			"details": err.Error(),
		})
		return
	}
	
	db := database.GetDB()
	if err := db.Model(&database.Plugin{}).
		Where("plugin_id = ?", pluginID).
		Update("config_data", string(configData)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update configuration",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":   "Plugin configuration updated successfully",
		"plugin_id": pluginID,
		"config":    info.Config,
	})
}

// =============================================================================
// PLUGIN INSTALLATION ENDPOINTS
// =============================================================================

// InstallPlugin installs a new plugin
func InstallPlugin(c *gin.Context) {
	var installRequest struct {
		Source string                 `json:"source" binding:"required"` // URL, file path, or plugin ID
		Config map[string]interface{} `json:"config,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&installRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}
	
	// For now, return not implemented
	// In a full implementation, this would download, extract, and install the plugin
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "Plugin installation not yet implemented",
		"source":  installRequest.Source,
	})
}

// UninstallPlugin uninstalls a plugin
func UninstallPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	// First disable the plugin if it's enabled
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if _, exists := pluginManager.GetPlugin(pluginID); exists {
		if err := pluginManager.DisablePlugin(ctx, pluginID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to disable plugin before uninstall",
				"details": err.Error(),
			})
			return
		}
	}
	
	// Remove from database
	db := database.GetDB()
	if err := db.Where("plugin_id = ?", pluginID).Delete(&database.Plugin{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to remove plugin from database",
			"details": err.Error(),
		})
		return
	}
	
	// For now, just remove from database
	// In a full implementation, this would also remove plugin files
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin uninstalled successfully",
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

// RefreshPlugins rediscovers available plugins
func RefreshPlugins(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := pluginManager.DiscoverPlugins(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to refresh plugins",
			"details": err.Error(),
		})
		return
	}
	
	plugins := pluginManager.ListPlugins()
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Plugins refreshed successfully",
		"count":   len(plugins),
		"plugins": plugins,
	})
}

// GetPluginManifest returns the manifest for a specific plugin
func GetPluginManifest(c *gin.Context) {
	pluginID := c.Param("id")
	
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Plugin manager not initialized",
		})
		return
	}
	
	info, exists := pluginManager.GetPluginInfo(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Plugin not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugin_id": pluginID,
		"manifest":  info.Manifest,
	})
}

// GetPluginAdminPages returns admin pages provided by plugins
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
