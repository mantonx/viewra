package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterEnrichmentRoutes registers the enrichment routes.
func RegisterEnrichmentRoutes(router *gin.RouterGroup, handler *handlers.EnrichmentHandler) {
	enrichment := router.Group("/enrichment")
	{
		// Queue statistics
		enrichment.GET("/stats", handler.GetStats)

		// SSE progress streaming
		enrichment.GET("/progress", handler.StreamProgress)

		// Manual enqueue
		enrichment.POST("/enqueue", handler.EnqueueMedia)
	}
}
