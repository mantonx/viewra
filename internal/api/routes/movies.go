package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterMoviesRoutes registers all movie-related routes
func RegisterMoviesRoutes(rg *gin.RouterGroup, handler *handlers.MoviesHandler) {
	movies := rg.Group("/movies")
	{
		movies.GET("", handler.List)           // GET /api/movies?library_id=1
		movies.GET("/search", handler.Search)  // GET /api/movies/search?library_id=1&q=batman
		movies.GET("/:id", handler.Get)        // GET /api/movies/123
	}
}
