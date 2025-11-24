package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterAdaptiveQualityRoutes registers adaptive quality recommendation routes.
func RegisterAdaptiveQualityRoutes(router *gin.Engine, logger *slog.Logger) {
	handler := handlers.NewAdaptiveQualityHandler(logger)

	api := router.Group("/api")
	{
		adaptive := api.Group("/adaptive")
		{
			// Quality recommendation
			adaptive.POST("/recommend", handler.RecommendQuality)

			// Adaptive bitrate ladder
			adaptive.POST("/ladder", handler.GetAdaptiveLadder)
		}

		speedtest := api.Group("/speedtest")
		{
			// Speed test chunk download
			speedtest.GET("/chunk", handler.SpeedTestChunk)
		}
	}
}
