package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterHomeRoutes registers home screen routes.
func RegisterHomeRoutes(router *gin.RouterGroup, handler *handlers.HomeHandler) {
	if handler == nil {
		return
	}

	// Main home endpoint
	router.GET("/home", handler.GetHome)

	// Section-specific endpoints
	router.GET("/home/sections/:id", handler.GetSection)

	// Preferences endpoints
	router.GET("/home/preferences", handler.GetPreferences)
	router.PUT("/home/preferences", handler.UpdatePreferences)
	router.DELETE("/home/preferences", handler.ResetPreferences)
}
