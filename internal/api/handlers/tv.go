package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/tv"
)

// TVHandler handles HTTP requests for TV shows and episodes
type TVHandler struct {
	listShows    tv.ListTVShowsExecutor
	getShow      tv.GetTVShowExecutor
	listEpisodes tv.ListTVEpisodesExecutor
	getEpisode   tv.GetTVEpisodeExecutor
	searchTV     tv.SearchTVEpisodesExecutor
}

// NewTVHandler creates a new TV handler
func NewTVHandler(
	listShows tv.ListTVShowsExecutor,
	getShow tv.GetTVShowExecutor,
	listEpisodes tv.ListTVEpisodesExecutor,
	getEpisode tv.GetTVEpisodeExecutor,
	searchTV tv.SearchTVEpisodesExecutor,
) *TVHandler {
	return &TVHandler{
		listShows:    listShows,
		getShow:      getShow,
		listEpisodes: listEpisodes,
		getEpisode:   getEpisode,
		searchTV:     searchTV,
	}
}

// ListShows handles GET /api/tv/shows
// @Summary List TV shows
// @Description Returns a list of all TV shows in a library with episode and season counts
// @Tags tv
// @Produce json
// @Param library_id query int true "Library ID to filter shows"
// @Success 200 {object} tv.ListTVShowsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows [get]
func (h *TVHandler) ListShows(c *gin.Context) {
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows/{id} [get]
func (h *TVHandler) GetShow(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid show ID",
			Message: err.Error(),
		})
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows/{showTitle}/episodes [get]
func (h *TVHandler) ListEpisodesByShow(c *gin.Context) {
	showTitle := c.Param("showTitle")
	if showTitle == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing show title",
			Message: "showTitle path parameter is required",
		})
		return
	}

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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/shows/{id}/episodes [get]
func (h *TVHandler) ListEpisodesByShowID(c *gin.Context) {
	showID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid show ID",
			Message: err.Error(),
		})
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/episodes/{id} [get]
func (h *TVHandler) GetEpisode(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid episode ID",
			Message: err.Error(),
		})
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
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/tv/search [get]
func (h *TVHandler) Search(c *gin.Context) {
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

	resp, err := h.searchTV.Execute(c.Request.Context(), libraryID, query)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
