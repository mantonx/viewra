package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterRatingsRoutes registers the ratings routes.
func RegisterRatingsRoutes(router *gin.RouterGroup, handler *handlers.RatingsHandler) {
	if handler == nil {
		return
	}

	ratings := router.Group("/ratings")
	{
		// List and create ratings
		ratings.GET("", handler.ListRatings)
		ratings.POST("", handler.CreateRating)

		// Get/delete specific rating
		ratings.GET("/:entity_type/:entity_id", handler.GetRating)
		ratings.DELETE("/:entity_type/:entity_id", handler.DeleteRating)
	}
}
