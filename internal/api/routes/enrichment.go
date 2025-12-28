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

		// Stage status including circuit breaker state
		enrichment.GET("/stages", handler.GetStages)
		enrichment.POST("/stages/:stage/reset", handler.ResetStageCircuitBreaker)

		// Manual enqueue
		enrichment.POST("/enqueue", handler.EnqueueMedia)

		// Bulk enqueue all media of a type for re-enrichment
		enrichment.POST("/bulk-enqueue", handler.BulkEnqueue)

		// Priority boost for interactive use (user viewing an item)
		enrichment.POST("/prioritize", handler.Prioritize)
	}
}
