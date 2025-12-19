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
		analytics.GET("/sessions", handler.ListSessions)
		analytics.GET("/sessions/:id", handler.GetSession)
		analytics.GET("/summary", handler.GetMediaSummary)
	}
}
