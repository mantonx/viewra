package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterSearchRoutes registers search routes.
// Plugins can override /api/search by registering the "search" capability.
func RegisterSearchRoutes(router *gin.RouterGroup, handler *handlers.SearchHandler) {
	if handler == nil {
		return
	}
	router.GET("/search", handler.Search)
}
