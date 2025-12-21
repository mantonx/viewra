package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterSettingsRoutes registers all settings-related routes.
// System settings require admin, user settings require authentication.
func RegisterSettingsRoutes(
	protected *gin.RouterGroup,
	handler *handlers.SettingsHandler,
	authValidator middleware.AuthValidator,
) {
	RegisterSettingsRoutesWithLocation(protected, handler, nil, authValidator)
}

// RegisterSettingsRoutesWithLocation registers all settings-related routes including location.
func RegisterSettingsRoutesWithLocation(
	protected *gin.RouterGroup,
	handler *handlers.SettingsHandler,
	locationHandler *handlers.LocationSettingsHandler,
	authValidator middleware.AuthValidator,
) {
	if handler == nil {
		return
	}

	settings := protected.Group("/settings")

	// User settings (authenticated users)
	user := settings.Group("/user")
	user.GET("", handler.GetAllUser)
	user.GET("/:key", handler.GetUser)
	user.PUT("/:key", handler.SetUser)
	user.DELETE("/:key", handler.DeleteUser)

	// Schema (authenticated users can see schema)
	settings.GET("/schema", handler.GetSchema)

	// System settings (admin only)
	system := settings.Group("/system")
	if authValidator != nil {
		system.Use(middleware.RequireAdmin(authValidator))
	}
	system.GET("", handler.GetAllSystem)
	system.GET("/effective", handler.GetAllSystemEffective) // Must be before /:key
	system.GET("/:key", handler.GetSystem)
	system.PUT("/:key", handler.SetSystem)

	// System info endpoint (admin only) - /api/system/info
	systemInfo := protected.Group("/system")
	if authValidator != nil {
		systemInfo.Use(middleware.RequireAdmin(authValidator))
	}
	systemInfo.GET("/info", handler.GetSystemInfo)

	// Location settings (authenticated users)
	if locationHandler != nil {
		location := settings.Group("/location")
		location.GET("", locationHandler.GetLocationPreferences)
		location.PUT("", locationHandler.UpdateLocationPreferences)

		// Weather context endpoint
		protected.GET("/context/weather", locationHandler.GetWeatherContext)
	}
}
