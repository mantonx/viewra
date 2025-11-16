package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterTVRoutes registers all TV-related routes
func RegisterTVRoutes(rg *gin.RouterGroup, handler *handlers.TVHandler) {
	tv := rg.Group("/tv")
	{
		// TV Shows
		tv.GET("/shows", handler.ListShows)                         // GET /api/tv/shows?library_id=1
		tv.GET("/shows/:id", handler.GetShow)                       // GET /api/tv/shows/3
		tv.GET("/shows/:id/episodes", handler.ListEpisodesByShowID) // GET /api/tv/shows/3/episodes

		// TV Episodes
		tv.GET("/episodes/:id", handler.GetEpisode) // GET /api/tv/episodes/123

		// Search
		tv.GET("/search", handler.Search) // GET /api/tv/search?library_id=1&q=breaking
	}
}
