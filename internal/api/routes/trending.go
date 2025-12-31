package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterTrendingRoutes registers trending data routes.
func RegisterTrendingRoutes(router *gin.RouterGroup, handler *handlers.TrendingHandler) {
	if handler == nil {
		return
	}

	router.GET("/trending", handler.GetTrending)
	router.GET("/trending/providers", handler.GetProviders)
}
