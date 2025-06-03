package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// CorePluginsHandler handles core plugin management
type CorePluginsHandler struct {
	pluginModule *pluginmodule.PluginModule
}

// NewCorePluginsHandler creates a new core plugins handler
func NewCorePluginsHandler(pluginModule *pluginmodule.PluginModule) *CorePluginsHandler {
	return &CorePluginsHandler{
		pluginModule: pluginModule,
	}
}

// ListCorePlugins returns all core plugins
func (h *CorePluginsHandler) ListCorePlugins(c *gin.Context) {
	if h.pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Plugin module not available",
		})
		return
	}

	plugins := h.pluginModule.GetCoreManager().ListCorePluginInfo()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plugins,
		"count":   len(plugins),
	})
}

// EnableCorePlugin enables a specific core plugin
func (h *CorePluginsHandler) EnableCorePlugin(c *gin.Context) {
	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Plugin name is required",
		})
		return
	}

	if h.pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Plugin module not available",
		})
		return
	}

	if err := h.pluginModule.EnableCorePlugin(pluginName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin enabled successfully",
		"plugin":  pluginName,
	})
}

// DisableCorePlugin disables a specific core plugin
func (h *CorePluginsHandler) DisableCorePlugin(c *gin.Context) {
	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Plugin name is required",
		})
		return
	}

	if h.pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Plugin module not available",
		})
		return
	}

	if err := h.pluginModule.DisableCorePlugin(pluginName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin disabled successfully",
		"plugin":  pluginName,
	})
}

// GetCorePluginInfo returns information about a specific core plugin
func (h *CorePluginsHandler) GetCorePluginInfo(c *gin.Context) {
	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Plugin name is required",
		})
		return
	}

	if h.pluginModule == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Plugin module not available",
		})
		return
	}

	allPlugins := h.pluginModule.GetCoreManager().ListCorePluginInfo()
	for _, plugin := range allPlugins {
		if plugin.Name == pluginName {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    plugin,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"error":   "Plugin not found",
	})
}
