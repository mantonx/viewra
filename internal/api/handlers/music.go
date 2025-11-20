package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/music"
	"github.com/viewra/viewra/internal/domain/common"
)

// MusicHandler handles HTTP requests for music
type MusicHandler struct {
	listArtists          music.ListArtistsExecutor
	listAlbumsByArtistID music.ListAlbumsByArtistIDExecutor
	listTracksByAlbumID  music.ListTracksByAlbumIDExecutor
	getTrack             music.GetTrackExecutor
	searchTracks         music.SearchTracksExecutor
}

// NewMusicHandler creates a new music handler
func NewMusicHandler(
	listArtists music.ListArtistsExecutor,
	listAlbumsByArtistID music.ListAlbumsByArtistIDExecutor,
	listTracksByAlbumID music.ListTracksByAlbumIDExecutor,
	getTrack music.GetTrackExecutor,
	searchTracks music.SearchTracksExecutor,
) *MusicHandler {
	return &MusicHandler{
		listArtists:          listArtists,
		listAlbumsByArtistID: listAlbumsByArtistID,
		listTracksByAlbumID:  listTracksByAlbumID,
		getTrack:             getTrack,
		searchTracks:         searchTracks,
	}
}

// ListArtists handles GET /api/music/artists
// @Summary List music artists
// @Description Returns a list of all artists in a library with album and track counts and optional pagination
// @Tags music
// @Produce json
// @Param library_id query int true "Library ID to filter artists"
// @Param limit query int false "Number of items per page (default: 50, max: 200)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Success 200 {object} music.ListArtistsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/artists [get]
func (h *MusicHandler) ListArtists(c *gin.Context) {
	libraryID, ok := getRequiredQueryInt64(c, "library_id")
	if !ok {
		return
	}

	// Parse optional pagination and sort parameters
	pagination := parsePaginationParams(c)
	sort := c.Query("sort")

	// If pagination parameters were provided, use paginated endpoint
	if c.Query("limit") != "" || c.Query("offset") != "" || sort != "" {
		paginationParams := common.NewPaginationParamsWithSort(pagination.limit, pagination.offset, sort)
		resp, err := h.listArtists.ExecuteWithPagination(c.Request.Context(), libraryID, paginationParams)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// Otherwise use non-paginated endpoint for backward compatibility
	resp, err := h.listArtists.Execute(c.Request.Context(), libraryID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTrack handles GET /api/music/tracks/:id
// @Summary Get a music track by ID
// @Description Returns details of a specific music track including all metadata
// @Tags music
// @Produce json
// @Param id path int true "Track ID"
// @Success 200 {object} music.MusicTrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/tracks/{id} [get]
func (h *MusicHandler) GetTrack(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid track ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.getTrack.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Search handles GET /api/music/search
// @Summary Search music
// @Description Searches for music tracks by title, artist, or album in a specific library
// @Tags music
// @Produce json
// @Param library_id query int true "Library ID to search in"
// @Param q query string true "Search query (title, artist, or album)"
// @Success 200 {object} music.ListTracksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/search [get]
func (h *MusicHandler) Search(c *gin.Context) {
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

	resp, err := h.searchTracks.Execute(c.Request.Context(), libraryID, query)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListAlbumsByArtistID handles GET /api/music/artists/:id/albums
// @Summary List albums for an artist by ID
// @Description Returns all albums for an artist identified by a representative track ID
// @Tags music
// @Produce json
// @Param id path int true "Artist representative track ID"
// @Success 200 {object} music.ListAlbumsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/artists/{id}/albums [get]
func (h *MusicHandler) ListAlbumsByArtistID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid artist ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.listAlbumsByArtistID.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListTracksByAlbumID handles GET /api/music/albums/:id/tracks
// @Summary List tracks for an album by ID
// @Description Returns all tracks for an album identified by a representative track ID
// @Tags music
// @Produce json
// @Param id path int true "Album representative track ID"
// @Success 200 {object} music.ListTracksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/albums/{id}/tracks [get]
func (h *MusicHandler) ListTracksByAlbumID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid album ID",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.listTracksByAlbumID.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
