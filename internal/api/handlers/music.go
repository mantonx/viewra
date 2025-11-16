package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/music"
)

// MusicHandler handles HTTP requests for music
type MusicHandler struct {
	listArtists  music.ListArtistsExecutor
	listAlbums   music.ListAlbumsExecutor
	listTracks   music.ListTracksExecutor
	getTrack     music.GetTrackExecutor
	searchTracks music.SearchTracksExecutor
}

// NewMusicHandler creates a new music handler
func NewMusicHandler(
	listArtists music.ListArtistsExecutor,
	listAlbums music.ListAlbumsExecutor,
	listTracks music.ListTracksExecutor,
	getTrack music.GetTrackExecutor,
	searchTracks music.SearchTracksExecutor,
) *MusicHandler {
	return &MusicHandler{
		listArtists:  listArtists,
		listAlbums:   listAlbums,
		listTracks:   listTracks,
		getTrack:     getTrack,
		searchTracks: searchTracks,
	}
}

// ListArtists handles GET /api/music/artists
// @Summary List music artists
// @Description Returns a list of all artists in a library with album and track counts
// @Tags music
// @Produce json
// @Param library_id query int true "Library ID to filter artists"
// @Success 200 {object} music.ListArtistsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/artists [get]
func (h *MusicHandler) ListArtists(c *gin.Context) {
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

	resp, err := h.listArtists.Execute(c.Request.Context(), libraryID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListAlbumsByArtist handles GET /api/music/artists/:artist/albums
// @Summary List albums for an artist
// @Description Returns all albums for a specific artist with track counts
// @Tags music
// @Produce json
// @Param artist path string true "Artist name"
// @Param library_id query int true "Library ID"
// @Success 200 {object} music.ListAlbumsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/artists/{artist}/albums [get]
func (h *MusicHandler) ListAlbumsByArtist(c *gin.Context) {
	artist := c.Param("artist")
	if artist == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing artist name",
			Message: "artist path parameter is required",
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

	resp, err := h.listAlbums.ExecuteByArtist(c.Request.Context(), libraryID, artist)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListTracksByAlbum handles GET /api/music/albums/:album/tracks
// @Summary List tracks for an album
// @Description Returns all tracks for a specific album
// @Tags music
// @Produce json
// @Param album path string true "Album name"
// @Param library_id query int true "Library ID"
// @Success 200 {object} music.ListTracksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/music/albums/{album}/tracks [get]
func (h *MusicHandler) ListTracksByAlbum(c *gin.Context) {
	album := c.Param("album")
	if album == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing album name",
			Message: "album path parameter is required",
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

	resp, err := h.listTracks.ExecuteByAlbum(c.Request.Context(), libraryID, album)
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
