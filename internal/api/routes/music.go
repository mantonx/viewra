package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterMusicRoutes registers all music-related routes
func RegisterMusicRoutes(rg *gin.RouterGroup, handler *handlers.MusicHandler) {
	music := rg.Group("/music")
	{
		// Artists
		music.GET("/artists", handler.ListArtists)                     // GET /api/music/artists?library_id=1
		music.GET("/artists/:id/albums", handler.ListAlbumsByArtistID) // GET /api/music/artists/123/albums

		// Albums
		music.GET("/albums/:id/tracks", handler.ListTracksByAlbumID) // GET /api/music/albums/123/tracks

		// Tracks
		music.GET("/tracks/:id", handler.GetTrack) // GET /api/music/tracks/123

		// IDs for prefetching
		music.GET("/ids", handler.ListIDs) // GET /api/music/ids?library_id=1

		// Search
		music.GET("/search", handler.Search) // GET /api/music/search?library_id=1&q=imagine
	}
}
