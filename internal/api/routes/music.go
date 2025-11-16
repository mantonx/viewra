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
		music.GET("/artists", handler.ListArtists)                       // GET /api/music/artists?library_id=1
		music.GET("/artists/:artist/albums", handler.ListAlbumsByArtist) // GET /api/music/artists/Beatles/albums?library_id=1

		// Albums
		music.GET("/albums/:album/tracks", handler.ListTracksByAlbum)  // GET /api/music/albums/Abbey%20Road/tracks?library_id=1

		// Tracks
		music.GET("/tracks/:id", handler.GetTrack)  // GET /api/music/tracks/123

		// Search
		music.GET("/search", handler.Search)  // GET /api/music/search?library_id=1&q=imagine
	}
}
