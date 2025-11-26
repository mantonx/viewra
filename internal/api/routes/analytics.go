package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterAnalyticsRoutes registers playback analytics routes.
func RegisterAnalyticsRoutes(router *gin.RouterGroup, handler *handlers.AnalyticsHandler) {
	analytics := router.Group("/analytics")
	{
		analytics.POST("/playback", handler.RecordPlaybackAnalytics)
	}
}
