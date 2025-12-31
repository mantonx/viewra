package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterSystemRoutes registers system management routes.
// These routes require admin authentication.
func RegisterSystemRoutes(admin *gin.RouterGroup, h *handlers.SystemHandler, authValidator middleware.AuthValidator) {
	if h == nil {
		return
	}

	// System group for restart and maintenance endpoints
	system := admin.Group("/system")
	{
		// Restart management (admin only)
		system.GET("/restart", h.GetRestartStatus)
		system.POST("/restart", h.RequestRestart)
		system.DELETE("/restart", h.CancelRestart)
		system.POST("/restart/now", h.ExecuteRestart)

		// Maintenance mode (admin only)
		system.GET("/maintenance", h.GetMaintenance)
		system.POST("/maintenance", h.SetMaintenance)

		// Database migration (admin only)
		database := system.Group("/database")
		{
			database.POST("/test-connection", h.TestDatabaseConnection)
			database.POST("/estimate", h.EstimateMigration)
			database.POST("/migrate", h.StartMigration)
			database.GET("/migrate", h.GetMigrationStatus)
		}
	}

	// Admin status stream (SSE)
	admin.GET("/status/stream", h.StreamAdminStatus)
}
