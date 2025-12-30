package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterSearchRoutes registers search routes with fallback support.
// The SearchHandler checks if a semantic search plugin is available and
// falls back to basic text search if not.
func RegisterSearchRoutes(router *gin.RouterGroup, handler *handlers.SearchHandler) {
	if handler == nil {
		return
	}
	router.GET("/search", handler.Search)
}
