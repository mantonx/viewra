package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all media module routes
func RegisterRoutes(router *gin.Engine, handler *Handler) {
	// Media file endpoints
	mediaGroup := router.Group("/api/media")
	{
		mediaGroup.GET("/files", handler.GetMediaFiles)
		mediaGroup.GET("/files/:id", handler.GetMediaFile)
		mediaGroup.GET("/files/:id/metadata", handler.GetMediaFileMetadata)
		mediaGroup.GET("/files/:id/stream", handler.StreamMediaFile)
		mediaGroup.HEAD("/files/:id/stream", handler.StreamMediaFile)
		mediaGroup.GET("/files/:id/album-artwork", handler.GetAlbumArtwork)
		mediaGroup.GET("/files/:id/album-id", handler.GetAlbumID)
		mediaGroup.GET("/music", handler.GetMusic) // Get music files
		mediaGroup.GET("/", handler.SearchMedia) // Search endpoint
	}
	
	// TV show endpoints
	tvGroup := router.Group("/api/tv")
	{
		tvGroup.GET("/shows", handler.GetTVShows)
		// TODO: Add more TV endpoints as needed
		// tvGroup.GET("/shows/:id", handler.GetTVShow)
		// tvGroup.GET("/shows/:id/seasons", handler.GetSeasons)
		// tvGroup.GET("/shows/:showId/seasons/:seasonId/episodes", handler.GetEpisodes)
		// tvGroup.GET("/episodes/:episodeId", handler.GetEpisode)
	}
	
	// Movie endpoints
	movieGroup := router.Group("/api/movies")
	{
		movieGroup.GET("/", handler.GetMovies)
		// TODO: Add more movie endpoints as needed
		// movieGroup.GET("/:id", handler.GetMovie)
	}
	
	// Music endpoints
	musicGroup := router.Group("/api/music")
	{
		musicGroup.GET("/albums", handler.GetAlbums)
		musicGroup.GET("/albums/:id", handler.GetAlbum)
		musicGroup.GET("/artists", handler.GetArtists)
		// TODO: Add more music endpoints as needed
		// musicGroup.GET("/artists/:name", handler.GetArtist)
		// musicGroup.GET("/artists/:name/albums", handler.GetArtistAlbums)
		// musicGroup.GET("/playlists", handler.GetPlaylists)
	}
	
	// Admin endpoints for library management are handled by the admin module
	// The media module should not register admin routes to avoid conflicts
	// adminGroup := router.Group("/api/admin")
	// {
	//	adminGroup.GET("/media-libraries/", handler.GetLibraries)
	//	adminGroup.POST("/media-libraries/", handler.CreateLibrary)
	//	adminGroup.DELETE("/media-libraries/:id", handler.DeleteLibrary)
	// }
}