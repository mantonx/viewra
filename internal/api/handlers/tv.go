package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/tv"
	"github.com/mantonx/viewra/internal/domain/common"
)

// TVHandler handles HTTP requests for TV shows and episodes
type TVHandler struct {
	listShows      tv.ListTVShowsExecutor
	getShow        tv.GetTVShowExecutor
	listEpisodes   tv.ListTVEpisodesExecutor
	getEpisode     tv.GetTVEpisodeExecutor
	searchTV       tv.SearchTVEpisodesExecutor
	listShowIDs    tv.ListTVShowIDsExecutor
	getNextEpisode tv.GetNextEpisodeExecutor
}

// NewTVHandler creates a new TV handler
func NewTVHandler(
	listShows tv.ListTVShowsExecutor,
	getShow tv.GetTVShowExecutor,
	listEpisodes tv.ListTVEpisodesExecutor,
	getEpisode tv.GetTVEpisodeExecutor,
	searchTV tv.SearchTVEpisodesExecutor,
	listShowIDs tv.ListTVShowIDsExecutor,
	getNextEpisode tv.GetNextEpisodeExecutor,
) *TVHandler {
	return &TVHandler{
		listShows:      listShows,
		getShow:        getShow,
		listEpisodes:   listEpisodes,
		getEpisode:     getEpisode,
		searchTV:       searchTV,
		listShowIDs:    listShowIDs,
		getNextEpisode: getNextEpisode,
	}
}

// ListShows handles GET /api/tv/shows
// @Summary List TV shows
// @Description Returns a list of all TV shows in a library with episode and season counts and optional pagination
// @Tags tv
// @Produce json
// @Param library_id query int true "Library ID to filter shows"
// @Param q query string false "Search query to filter shows by title"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Success 200 {object} tv.ListTVShowsResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/shows [get]
func (h *TVHandler) ListShows(c *gin.Context) {
	libraryID, ok := getRequiredQueryInt64(c, "library_id")
	if !ok {
		return
	}

	// Parse optional pagination and sort parameters
	pagination := parsePaginationParams(c)
	sort := c.Query("sort")
	query := c.Query("q")

	// If search query is provided, use search endpoint
	if query != "" {
		paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sort)
		resp, err := h.listShows.ExecuteWithSearch(c.Request.Context(), libraryID, query, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// If pagination parameters were provided, use paginated endpoint
	if c.Query("limit") != "" || c.Query("offset") != "" || sort != "" {
		paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sort)
		resp, err := h.listShows.ExecuteWithPagination(c.Request.Context(), libraryID, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// Otherwise use non-paginated endpoint for backward compatibility
	resp, err := h.listShows.Execute(c.Request.Context(), libraryID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetShow handles GET /api/tv/shows/:id
// @Summary Get a TV show by ID
// @Description Returns details of a specific TV show including season and episode counts
// @Tags tv
// @Produce json
// @Param id path int true "Show ID"
// @Success 200 {object} tv.TVShowDetailResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/shows/{id} [get]
func (h *TVHandler) GetShow(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_SHOW_ID", err.Error())
		return
	}

	resp, err := h.getShow.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListEpisodesByShow handles GET /api/tv/shows/:showTitle/episodes
// @Summary List episodes for a TV show
// @Description Returns all episodes for a specific TV show
// @Tags tv
// @Produce json
// @Param showTitle path string true "Show title"
// @Param library_id query int true "Library ID"
// @Success 200 {object} tv.ListTVEpisodesResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/shows/{showTitle}/episodes [get]
func (h *TVHandler) ListEpisodesByShow(c *gin.Context) {
	showTitle := c.Param("showTitle")
	if showTitle == "" {
		respondError(c, http.StatusBadRequest, "MISSING_SHOW_TITLE", "showTitle path parameter is required")
		return
	}

	libraryIDStr := c.Query("library_id")
	if libraryIDStr == "" {
		respondError(c, http.StatusBadRequest, "MISSING_LIBRARY_ID", "library_id query parameter is required")
		return
	}

	libraryID, err := parseID(libraryIDStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	resp, err := h.listEpisodes.ExecuteByShow(c.Request.Context(), libraryID, showTitle)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListEpisodesByShowID handles GET /api/tv/shows/:id/episodes
// @Summary List episodes for a TV show by ID
// @Description Returns all episodes for a specific TV show using its ID
// @Tags tv
// @Produce json
// @Param id path int true "Show ID"
// @Success 200 {object} tv.ListTVEpisodesResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/shows/{id}/episodes [get]
func (h *TVHandler) ListEpisodesByShowID(c *gin.Context) {
	showID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_SHOW_ID", err.Error())
		return
	}

	resp, err := h.listEpisodes.ExecuteByShowID(c.Request.Context(), showID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetEpisode handles GET /api/tv/episodes/:id
// @Summary Get a TV episode by ID
// @Description Returns details of a specific TV episode including all metadata
// @Tags tv
// @Produce json
// @Param id path int true "Episode ID"
// @Success 200 {object} tv.TVEpisodeResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/episodes/{id} [get]
func (h *TVHandler) GetEpisode(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_EPISODE_ID", err.Error())
		return
	}

	resp, err := h.getEpisode.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Search handles GET /api/tv/search
// @Summary Search TV shows and episodes
// @Description Searches for TV shows and episodes by title in a specific library
// @Tags tv
// @Produce json
// @Param library_id query int true "Library ID to search in"
// @Param q query string true "Search query (show title or episode title)"
// @Success 200 {object} tv.ListTVEpisodesResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/search [get]
func (h *TVHandler) Search(c *gin.Context) {
	libraryIDStr := c.Query("library_id")
	if libraryIDStr == "" {
		respondError(c, http.StatusBadRequest, "MISSING_LIBRARY_ID", "library_id query parameter is required")
		return
	}

	libraryID, err := parseID(libraryIDStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_SEARCH_QUERY", "q query parameter is required")
		return
	}

	resp, err := h.searchTV.Execute(c.Request.Context(), libraryID, query)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListIDs handles GET /api/tv/ids
// @Summary List TV show IDs
// @Description Returns a list of TV show IDs only for prefetching images with optional pagination
// @Tags tv
// @Produce json
// @Param library_id query int true "Library ID to filter shows"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Param sort query string false "Sort order: title_asc or title_desc (default: title_asc)"
// @Success 200 {object} tv.ListIDsResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/ids [get]
func (h *TVHandler) ListIDs(c *gin.Context) {
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
	resp, err := h.listShowIDs.Execute(c.Request.Context(), libraryID, paginationParams)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetNextEpisode handles GET /api/tv/shows/:id/next-episode
// @Summary Get next episode to watch for a show
// @Description Returns the next episode to watch for a show based on watch progress. Returns in-progress episode if exists, otherwise first unwatched episode, or first episode if all watched
// @Tags tv
// @Produce json
// @Param id path int true "Show ID"
// @Success 200 {object} tv.TVEpisodeResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/tv/shows/{id}/next-episode [get]
func (h *TVHandler) GetNextEpisode(c *gin.Context) {
	showID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_SHOW_ID", err.Error())
		return
	}

	resp, err := h.getNextEpisode.Execute(c.Request.Context(), showID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
