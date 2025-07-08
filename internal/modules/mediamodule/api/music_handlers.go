// Package api - Music and audio handlers
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// GetArtists handles GET /api/music/artists
// It returns all artists in the music library.
//
// Query parameters:
//   - search: Search in artist names
//   - limit: Number of results per page
//   - offset: Number of results to skip
//   - sort_by: Field to sort by (name, album_count, track_count)
//   - sort_order: Sort direction (asc, desc)
//
// Response: Array of Artist objects with album and track counts
func (h *Handler) GetArtists(c *gin.Context) {
	var artists []database.Artist
	db := c.MustGet("db").(*gorm.DB)
	
	// Build query
	query := db.Model(&database.Artist{})
	
	// Search filter
	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}
	
	// Sorting
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	query = query.Order(sortBy + " " + sortOrder)
	
	// Pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			query = query.Limit(limit)
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			query = query.Offset(offset)
		}
	}
	
	if err := query.Find(&artists).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Enrich with album and track counts
	artistsWithCounts := h.enrichArtistsWithCounts(db, artists)
	
	c.JSON(http.StatusOK, artistsWithCounts)
}

// GetArtist handles GET /api/music/artists/:id
// It returns detailed information about a specific artist.
//
// Path parameters:
//   - id: The artist ID
//
// Response: Artist object with albums and track count or 404 if not found
func (h *Handler) GetArtist(c *gin.Context) {
	id := c.Param("id")
	
	var artist database.Artist
	db := c.MustGet("db").(*gorm.DB)
	
	// Load artist
	if err := db.Where("id = ?", id).First(&artist).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Artist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Get album and track counts
	var albumCount int64
	var trackCount int64
	db.Model(&database.Album{}).Where("artist_id = ?", id).Count(&albumCount)
	db.Model(&database.Track{}).Where("artist_id = ?", id).Count(&trackCount)
	
	// Get albums for this artist
	var albums []database.Album
	db.Where("artist_id = ?", id).Find(&albums)
	
	c.JSON(http.StatusOK, gin.H{
		"id":          artist.ID,
		"name":        artist.Name,
		"description": artist.Description,
		"image":       artist.Image,
		"albums":      albums,
		"album_count": albumCount,
		"track_count": trackCount,
		"created_at":  artist.CreatedAt,
		"updated_at":  artist.UpdatedAt,
	})
}

// GetArtistAlbums handles GET /api/music/artists/:id/albums
// It returns all albums for a specific artist.
//
// Path parameters:
//   - id: The artist ID
//
// Query parameters:
//   - sort_by: Field to sort by (title, year, track_count)
//   - sort_order: Sort direction (asc, desc)
//
// Response: Array of Album objects with track information
func (h *Handler) GetArtistAlbums(c *gin.Context) {
	artistID := c.Param("id")
	
	var albums []database.Album
	db := c.MustGet("db").(*gorm.DB)
	
	// Build query
	query := db.Preload("Tracks").Where("artist_id = ?", artistID)
	
	// Sorting
	sortBy := c.DefaultQuery("sort_by", "year")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	query = query.Order(sortBy + " " + sortOrder)
	
	if err := query.Find(&albums).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Enrich albums with additional info
	albumsWithInfo := h.enrichAlbumsWithInfo(db, albums)
	
	c.JSON(http.StatusOK, albumsWithInfo)
}

// GetAlbums handles GET /api/music/albums
// It returns all albums in the music library.
//
// Query parameters:
//   - artist_id: Filter by artist ID
//   - year: Filter by release year
//   - genre: Filter by genre
//   - search: Search in album titles
//   - limit: Number of results per page
//   - offset: Number of results to skip
//
// Response: Array of Album objects with artist and track information
func (h *Handler) GetAlbums(c *gin.Context) {
	var albums []database.Album
	db := c.MustGet("db").(*gorm.DB)
	
	// Build query with artist preload
	query := db.Preload("Artist").Model(&database.Album{})
	
	// Artist filter
	if artistID := c.Query("artist_id"); artistID != "" {
		query = query.Where("artist_id = ?", artistID)
	}
	
	// Year filter - extract year from release_date
	if year := c.Query("year"); year != "" {
		query = query.Where("YEAR(release_date) = ?", year)
	}
	
	// Search filter
	if search := c.Query("search"); search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}
	
	// Pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			query = query.Limit(limit)
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			query = query.Offset(offset)
		}
	}
	
	if err := query.Find(&albums).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Enrich with track counts and duration
	albumsWithInfo := h.enrichAlbumsWithInfo(db, albums)
	
	c.JSON(http.StatusOK, albumsWithInfo)
}

// GetAlbum handles GET /api/music/albums/:id
// It returns detailed information about a specific album.
//
// Path parameters:
//   - id: The album ID
//
// Response: Album object with artist, tracks, and media files or 404 if not found
func (h *Handler) GetAlbum(c *gin.Context) {
	id := c.Param("id")
	
	var album database.Album
	db := c.MustGet("db").(*gorm.DB)
	
	// Load album with artist
	if err := db.Preload("Artist").Where("id = ?", id).First(&album).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Get tracks for this album
	var tracks []database.Track
	db.Where("album_id = ?", id).Find(&tracks)
	
	// Attach media files to tracks
	tracksWithMedia := h.attachMediaFilesToTracks(db, tracks)
	
	// Calculate total duration
	var totalDuration int
	for _, track := range tracks {
		totalDuration += track.Duration
	}
	
	c.JSON(http.StatusOK, gin.H{
		"id":            album.ID,
		"title":         album.Title,
		"artist":        album.Artist,
		"artist_id":     album.ArtistID,
		"release_date":  album.ReleaseDate,
		"artwork":       album.Artwork,
		"tracks":        tracksWithMedia,
		"track_count":   len(tracks),
		"total_duration": totalDuration,
		"created_at":    album.CreatedAt,
		"updated_at":    album.UpdatedAt,
	})
}

// GetPlaylists handles GET /api/music/playlists
// It returns all playlists for the current user.
//
// Query parameters:
//   - user_id: Filter by user ID (admin only)
//   - limit: Number of results per page
//   - offset: Number of results to skip
//
// Response: Array of Playlist objects with track counts
func (h *Handler) GetPlaylists(c *gin.Context) {
	// TODO: Implement playlists when database schema is added
	c.JSON(http.StatusOK, gin.H{
		"playlists": []interface{}{},
		"count": 0,
	})
}

// GetPlaylist handles GET /api/music/playlists/:id
// It returns detailed information about a specific playlist.
//
// Path parameters:
//   - id: The playlist ID
//
// Response: Playlist object with tracks and media files or 404 if not found
func (h *Handler) GetPlaylist(c *gin.Context) {
	// TODO: Implement playlists when database schema is added
	c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
}

// CreatePlaylist handles POST /api/music/playlists
// It creates a new playlist for the current user.
//
// Request body:
//   {
//     "name": "My Playlist",
//     "description": "A great playlist",
//     "is_public": false
//   }
//
// Response: Created Playlist object
func (h *Handler) CreatePlaylist(c *gin.Context) {
	// TODO: Implement playlists when database schema is added
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Playlists not implemented yet"})
}

// AddTrackToPlaylist handles POST /api/music/playlists/:id/tracks
// It adds a track to a playlist.
//
// Path parameters:
//   - id: The playlist ID
//
// Request body:
//   {
//     "track_id": "123"
//   }
//
// Response: Success message
func (h *Handler) AddTrackToPlaylist(c *gin.Context) {
	// TODO: Implement playlists when database schema is added
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Playlists not implemented yet"})
}

// Helper functions

// enrichArtistsWithCounts adds album and track counts to artists
func (h *Handler) enrichArtistsWithCounts(db *gorm.DB, artists []database.Artist) []gin.H {
	result := make([]gin.H, len(artists))
	
	for i, artist := range artists {
		var albumCount int64
		var trackCount int64
		
		db.Model(&database.Album{}).Where("artist_id = ?", artist.ID).Count(&albumCount)
		db.Model(&database.Track{}).Where("artist_id = ?", artist.ID).Count(&trackCount)
		
		result[i] = gin.H{
			"id":          artist.ID,
			"name":        artist.Name,
			"description": artist.Description,
			"image":       artist.Image,
			"album_count": albumCount,
			"track_count": trackCount,
			"created_at":  artist.CreatedAt,
			"updated_at":  artist.UpdatedAt,
		}
	}
	
	return result
}

// enrichAlbumsWithInfo adds track count and total duration to albums
func (h *Handler) enrichAlbumsWithInfo(db *gorm.DB, albums []database.Album) []gin.H {
	result := make([]gin.H, len(albums))
	
	for i, album := range albums {
		var tracks []database.Track
		db.Where("album_id = ?", album.ID).Find(&tracks)
		
		var totalDuration int
		for _, track := range tracks {
			totalDuration += track.Duration
		}
		
		result[i] = gin.H{
			"id":             album.ID,
			"title":          album.Title,
			"artist":         album.Artist,
			"artist_id":      album.ArtistID,
			"release_date":   album.ReleaseDate,
			"artwork":        album.Artwork,
			"track_count":    len(tracks),
			"total_duration": totalDuration,
			"created_at":     album.CreatedAt,
			"updated_at":     album.UpdatedAt,
		}
	}
	
	return result
}



// attachMediaFilesToTracks attaches media file information to tracks
func (h *Handler) attachMediaFilesToTracks(db *gorm.DB, tracks []database.Track) []gin.H {
	result := make([]gin.H, len(tracks))
	
	for i, track := range tracks {
		trackData := gin.H{
			"id":           track.ID,
			"title":        track.Title,
			"artist":       track.Artist,
			"artist_id":    track.ArtistID,
			"album":        track.Album,
			"album_id":     track.AlbumID,
			"track_number": track.TrackNumber,
			"duration":     track.Duration,

			"created_at":   track.CreatedAt,
			"updated_at":   track.UpdatedAt,
		}
		
		// Find associated media file
		var mediaFile database.MediaFile
		if err := db.Where("media_id = ? AND media_type = ?", track.ID, "track").First(&mediaFile).Error; err == nil {
			trackData["media_file"] = mediaFile
		} else {
			trackData["media_file"] = nil
		}
		
		result[i] = trackData
	}
	
	return result
}