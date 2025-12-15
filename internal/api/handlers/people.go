package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/people"
)

// PeopleHandler handles HTTP requests for people and credits.
type PeopleHandler struct {
	getPerson           people.GetPersonExecutor
	getCreditsForEntity people.GetCreditsForEntityExecutor
	getCreditsForPerson people.GetCreditsForPersonExecutor
}

// NewPeopleHandler creates a new people handler.
func NewPeopleHandler(
	getPerson people.GetPersonExecutor,
	getCreditsForEntity people.GetCreditsForEntityExecutor,
	getCreditsForPerson people.GetCreditsForPersonExecutor,
) *PeopleHandler {
	return &PeopleHandler{
		getPerson:           getPerson,
		getCreditsForEntity: getCreditsForEntity,
		getCreditsForPerson: getCreditsForPerson,
	}
}

// GetPerson handles GET /api/people/:id
// @Summary Get a person by ID
// @Description Returns details of a specific person (actor, director, etc.)
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Success 200 {object} people.PersonResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/people/{id} [get]
func (h *PeopleHandler) GetPerson(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid person ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getPerson.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPersonCredits handles GET /api/people/:id/credits
// @Summary Get a person's credits (filmography)
// @Description Returns all credits for a specific person
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Success 200 {object} people.PersonCreditsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/people/{id}/credits [get]
func (h *PeopleHandler) GetPersonCredits(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid person ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getCreditsForPerson.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMovieCredits handles GET /api/movies/:id/credits
// @Summary Get credits for a movie
// @Description Returns cast, directors, and writers for a specific movie
// @Tags movies
// @Produce json
// @Param id path int true "Movie ID"
// @Success 200 {object} people.CreditsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/movies/{id}/credits [get]
func (h *PeopleHandler) GetMovieCredits(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid movie ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getCreditsForEntity.Execute(c.Request.Context(), "movie", id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTVShowCredits handles GET /api/tv/shows/:id/credits
// @Summary Get credits for a TV show
// @Description Returns cast, creators, and other credits for a specific TV show
// @Tags tv
// @Produce json
// @Param id path int true "TV Show ID"
// @Success 200 {object} people.CreditsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows/{id}/credits [get]
func (h *PeopleHandler) GetTVShowCredits(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid TV show ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getCreditsForEntity.Execute(c.Request.Context(), "tv_show", id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTVEpisodeCredits handles GET /api/tv/episodes/:id/credits
// @Summary Get credits for a TV episode
// @Description Returns cast and crew for a specific TV episode
// @Tags tv
// @Produce json
// @Param id path int true "TV Episode ID"
// @Success 200 {object} people.CreditsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/episodes/{id}/credits [get]
func (h *PeopleHandler) GetTVEpisodeCredits(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid TV episode ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getCreditsForEntity.Execute(c.Request.Context(), "tv_episode", id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
