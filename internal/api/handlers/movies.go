package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/movies"
)

// MoviesHandler handles HTTP requests for movies
type MoviesHandler struct {
	listMovies   movies.ListMoviesExecutor
	getMovie     movies.GetMovieExecutor
	searchMovies movies.SearchMoviesExecutor
}

// NewMoviesHandler creates a new movies handler
func NewMoviesHandler(
	listMovies movies.ListMoviesExecutor,
	getMovie movies.GetMovieExecutor,
	searchMovies movies.SearchMoviesExecutor,
) *MoviesHandler {
	return &MoviesHandler{
		listMovies:   listMovies,
		getMovie:     getMovie,
		searchMovies: searchMovies,
	}
}

// List handles GET /api/movies
// @Summary List movies
// @Description Returns a list of all movies in a specific library
// @Tags movies
// @Produce json
// @Param library_id query int true "Library ID to filter movies"
// @Success 200 {object} movies.ListMoviesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/movies [get]
func (h *MoviesHandler) List(c *gin.Context) {
	libraryIDStr := c.Query("library_id")
	if libraryIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing library_id",
			Message: "library_id query parameter is required",
		})
		return
	}

	libraryID, err := parseID(libraryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library_id",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.listMovies.Execute(c.Request.Context(), libraryID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/movies/:id
// @Summary Get a movie by ID
// @Description Returns details of a specific movie including all metadata
// @Tags movies
// @Produce json
// @Param id path int true "Movie ID"
// @Success 200 {object} movies.MovieResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/movies/{id} [get]
func (h *MoviesHandler) Get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid movie ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getMovie.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Search handles GET /api/movies/search
// @Summary Search movies
// @Description Searches for movies by title in a specific library
// @Tags movies
// @Produce json
// @Param library_id query int true "Library ID to search in"
// @Param q query string true "Search query (title)"
// @Success 200 {object} movies.ListMoviesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/movies/search [get]
func (h *MoviesHandler) Search(c *gin.Context) {
	libraryIDStr := c.Query("library_id")
	if libraryIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing library_id",
			Message: "library_id query parameter is required",
		})
		return
	}

	libraryID, err := parseID(libraryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid library_id",
			Message: err.Error(),
		})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing search query",
			Message: "q query parameter is required",
		})
		return
	}

	resp, err := h.searchMovies.Execute(c.Request.Context(), libraryID, query)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
