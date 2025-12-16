package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterPluginRoutes registers all plugin management routes.
// Read operations (list, get, settings, health) require authentication.
// Write operations (enable, disable, restart, update settings, logs) require admin.
func RegisterPluginRoutes(
	protected *gin.RouterGroup,
	handler *handlers.PluginHandler,
	authValidator middleware.AuthValidator,
) {
	if handler == nil {
		return
	}

	plugins := protected.Group("/plugins")

	// Read-only endpoints (authenticated users)
	plugins.GET("", handler.List)
	plugins.GET("/:id", handler.Get)
	plugins.GET("/:id/settings", handler.GetSettings)
	plugins.GET("/:id/health", handler.GetHealth)

	// Admin-only endpoints
	admin := plugins.Group("")
	if authValidator != nil {
		admin.Use(middleware.RequireAdmin(authValidator))
	}
	admin.PUT("/:id/settings", handler.UpdateSettings)
	admin.POST("/:id/enable", handler.Enable)
	admin.POST("/:id/disable", handler.Disable)
	admin.POST("/:id/restart", handler.Restart)
	admin.GET("/:id/logs", handler.GetLogs)
}
