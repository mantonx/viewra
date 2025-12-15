package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterPeopleRoutes registers all people-related routes.
func RegisterPeopleRoutes(rg *gin.RouterGroup, handler *handlers.PeopleHandler) {
	people := rg.Group("/people")
	{
		people.GET("/:id", handler.GetPerson)         // GET /api/people/123
		people.GET("/:id/credits", handler.GetPersonCredits) // GET /api/people/123/credits
	}
}

// RegisterCreditsRoutes registers credits routes on movie and TV endpoints.
// This function should be called after the movie and TV route groups are created.
func RegisterCreditsRoutes(moviesGroup, tvShowsGroup, tvEpisodesGroup *gin.RouterGroup, handler *handlers.PeopleHandler) {
	// Movie credits: GET /api/movies/:id/credits
	if moviesGroup != nil {
		moviesGroup.GET("/:id/credits", handler.GetMovieCredits)
	}

	// TV show credits: GET /api/tv/shows/:id/credits
	if tvShowsGroup != nil {
		tvShowsGroup.GET("/:id/credits", handler.GetTVShowCredits)
	}

	// TV episode credits: GET /api/tv/episodes/:id/credits
	if tvEpisodesGroup != nil {
		tvEpisodesGroup.GET("/:id/credits", handler.GetTVEpisodeCredits)
	}
}
