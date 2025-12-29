package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/domain/common"
)

// MoviesHandler handles HTTP requests for movies
type MoviesHandler struct {
	listMovies   movies.ListMoviesExecutor
	getMovie     movies.GetMovieExecutor
	searchMovies movies.SearchMoviesExecutor
	listMovieIDs movies.ListMovieIDsExecutor
}

// NewMoviesHandler creates a new movies handler
func NewMoviesHandler(
	listMovies movies.ListMoviesExecutor,
	getMovie movies.GetMovieExecutor,
	searchMovies movies.SearchMoviesExecutor,
	listMovieIDs movies.ListMovieIDsExecutor,
) *MoviesHandler {
	return &MoviesHandler{
		listMovies:   listMovies,
		getMovie:     getMovie,
		searchMovies: searchMovies,
		listMovieIDs: listMovieIDs,
	}
}

// List handles GET /api/movies
// @Summary List movies
// @Description Returns a list of all movies in a specific library with optional pagination
// @Tags movies
// @Produce json
// @Param library_id query int true "Library ID to filter movies"
// @Param q query string false "Search query to filter movies by title"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Param sort query string false "Sort order: title_asc or title_desc (default: title_asc)"
// @Success 200 {object} movies.ListMoviesResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/movies [get]
func (h *MoviesHandler) List(c *gin.Context) {
	libraryID, ok := getRequiredQueryInt64(c, "library_id")
	if !ok {
		return
	}

	// Parse optional pagination parameters
	pagination := parsePaginationParams(c)

	// Parse optional sort parameter
	sortBy := c.Query("sort")
	if sortBy == "" {
		sortBy = "title_asc"
	}

	// Parse optional search query
	query := c.Query("q")

	// If search query is provided, use search endpoint
	if query != "" {
		paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sortBy)
		resp, err := h.searchMovies.ExecuteWithPagination(c.Request.Context(), libraryID, query, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// If pagination parameters were provided, use paginated endpoint
	if c.Query("limit") != "" || c.Query("offset") != "" {
		paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sortBy)
		resp, err := h.listMovies.ExecuteWithPagination(c.Request.Context(), libraryID, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// Otherwise use non-paginated endpoint for backward compatibility
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
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
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
// @Description Searches for movies by title in a specific library with optional pagination
// @Tags movies
// @Produce json
// @Param library_id query int true "Library ID to search in"
// @Param q query string true "Search query (title)"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Success 200 {object} movies.ListMoviesResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/movies/search [get]
func (h *MoviesHandler) Search(c *gin.Context) {
	libraryID, ok := getRequiredQueryInt64(c, "library_id")
	if !ok {
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

	// Parse optional pagination parameters
	pagination := parsePaginationParams(c)

	// If pagination parameters were provided, use paginated endpoint
	if c.Query("limit") != "" || c.Query("offset") != "" {
		paginationParams := common.NewPaginationParams(pagination.limit, pagination.offset)
		resp, err := h.searchMovies.ExecuteWithPagination(c.Request.Context(), libraryID, query, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// Otherwise use non-paginated endpoint for backward compatibility
	resp, err := h.searchMovies.Execute(c.Request.Context(), libraryID, query)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListIDs handles GET /api/movies/ids
// @Summary List movie IDs
// @Description Returns a list of movie IDs only for prefetching images with optional pagination
// @Tags movies
// @Produce json
// @Param library_id query int true "Library ID to filter movies"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Param sort query string false "Sort order: title_asc or title_desc (default: title_asc)"
// @Success 200 {object} movies.ListIDsResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/movies/ids [get]
func (h *MoviesHandler) ListIDs(c *gin.Context) {
	libraryID, ok := getRequiredQueryInt64(c, "library_id")
	if !ok {
		return
	}

	// Parse optional pagination parameters
	pagination := parsePaginationParams(c)

	// Parse optional sort parameter
	sortBy := c.Query("sort")
	if sortBy == "" {
		sortBy = "title_asc"
	}

	paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sortBy)
	resp, err := h.listMovieIDs.Execute(c.Request.Context(), libraryID, paginationParams)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
