package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// RegisterPluginRoutes registers all plugin management routes.
// Read operations (list, get, settings, health) require authentication.
// Write operations (enable, disable, restart, update settings, logs) require admin.
func RegisterPluginRoutes(
	protected *gin.RouterGroup,
	handler *handlers.PluginHandler,
	proxy *plugins.HTTPProxy,
	authValidator middleware.AuthValidator,
) {
	if handler == nil {
		return
	}

	pluginsGroup := protected.Group("/plugins")

	// Read-only endpoints (authenticated users)
	pluginsGroup.GET("", handler.List)
	pluginsGroup.GET("/:id", handler.Get)
	pluginsGroup.GET("/:id/settings", handler.GetSettings)
	pluginsGroup.GET("/:id/health", handler.GetHealth)

	// Admin-only endpoints
	admin := pluginsGroup.Group("")
	if authValidator != nil {
		admin.Use(middleware.RequireAdmin(authValidator))
	}
	admin.PUT("/:id/settings", handler.UpdateSettings)
	admin.POST("/:id/enable", handler.Enable)
	admin.POST("/:id/disable", handler.Disable)
	admin.POST("/:id/restart", handler.Restart)
	admin.GET("/:id/logs", handler.GetLogs)

	// Plugin custom routes - handled by HTTP proxy
	// These allow plugins to expose their own HTTP endpoints
	// Route: /api/plugin/:plugin_id/*path (note: singular "plugin", not "plugins")
	// This avoids conflicts with the management routes above.
	if proxy != nil {
		pluginProxy := protected.Group("/plugin")
		pluginProxy.Any("/:plugin_id/*path", proxy.HandlePluginRoute)
	}
}
