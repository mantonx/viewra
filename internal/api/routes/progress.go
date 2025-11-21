package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterProgressRoutes registers the progress routes
func RegisterProgressRoutes(router *gin.RouterGroup, handler *handlers.ProgressHandler) {
	progress := router.Group("/progress")
	{
		// List and update progress
		progress.GET("", handler.ListProgress)
		progress.PUT("", handler.UpdateProgress)

		// Batch progress
		progress.GET("/batch", handler.GetBatchProgress)

		// Filtered lists
		progress.GET("/watched", handler.ListWatched)
		progress.GET("/in-progress", handler.ListInProgress)

		// Mark watched/unwatched
		progress.POST("/mark-watched", handler.MarkWatched)
		progress.POST("/mark-unwatched", handler.MarkUnwatched)

		// Get/delete progress for specific media
		progress.GET("/:media_id", handler.GetProgress)
		progress.DELETE("/:media_id", handler.DeleteProgress)
	}
}
