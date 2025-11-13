package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterBrowserRoutes registers filesystem browser routes
func RegisterBrowserRoutes(router *gin.RouterGroup, handler *handlers.BrowserHandler) {
	filesystem := router.Group("/filesystem")
	{
		filesystem.GET("/browse", handler.Browse)
	}
}
