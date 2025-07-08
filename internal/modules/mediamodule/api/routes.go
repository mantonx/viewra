package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all media module routes with comprehensive organization.
// Routes are grouped by domain: media files, TV shows, movies, music, and libraries.
func RegisterRoutes(router *gin.Engine, handler *Handler) {
	// Media file endpoints - core file operations
	mediaGroup := router.Group("/api/media")
	{
		mediaGroup.GET("/files", handler.GetMediaFiles)
		mediaGroup.GET("/files/:id", handler.GetMediaFile)
		mediaGroup.GET("/files/:id/metadata", handler.GetMediaFileMetadata)

		mediaGroup.GET("/files/:id/album-artwork", handler.GetAlbumArtwork)
		mediaGroup.GET("/files/:id/album-id", handler.GetAlbumID)
		mediaGroup.GET("/music", handler.GetMusic) // Get music files
		mediaGroup.GET("/", handler.SearchMedia) // Search endpoint
	}
	
	// TV show endpoints - comprehensive TV show management
	tvGroup := router.Group("/api/tv")
	{
		tvGroup.GET("/shows", handler.GetTVShows)
		tvGroup.GET("/shows/:id", handler.GetTVShow)
		tvGroup.GET("/shows/:id/seasons", handler.GetSeasons)
		tvGroup.GET("/shows/:id/seasons/:seasonId/episodes", handler.GetEpisodes)
		tvGroup.GET("/episodes/:episodeId", handler.GetEpisode)
	}
	
	// Movie endpoints - movie library operations
	movieGroup := router.Group("/api/movies")
	{
		movieGroup.GET("/", handler.GetMovies)
		movieGroup.GET("/search", handler.SearchMovies)
		movieGroup.GET("/:id", handler.GetMovie)
		movieGroup.GET("/:id/similar", handler.GetSimilarMovies)
	}
	
	// Music endpoints - comprehensive music library
	musicGroup := router.Group("/api/music")
	{
		// Artist endpoints
		musicGroup.GET("/artists", handler.GetArtists)
		musicGroup.GET("/artists/:id", handler.GetArtist)
		musicGroup.GET("/artists/:id/albums", handler.GetArtistAlbums)
		
		// Album endpoints
		musicGroup.GET("/albums", handler.GetAlbums)
		musicGroup.GET("/albums/:id", handler.GetAlbum)
		
		// Playlist endpoints
		musicGroup.GET("/playlists", handler.GetPlaylists)
		musicGroup.GET("/playlists/:id", handler.GetPlaylist)
		musicGroup.POST("/playlists", handler.CreatePlaylist)
		musicGroup.POST("/playlists/:id/tracks", handler.AddTrackToPlaylist)
	}
	
	// Library management endpoints
	libraryGroup := router.Group("/api/libraries")
	{
		libraryGroup.GET("/", handler.GetLibraries)
		libraryGroup.GET("/:id", handler.GetLibrary)
		libraryGroup.POST("/", handler.CreateLibrary)
		libraryGroup.PUT("/:id", handler.UpdateLibrary)
		libraryGroup.DELETE("/:id", handler.DeleteLibrary)
		libraryGroup.POST("/:id/scan", handler.ScanLibrary)
		libraryGroup.GET("/:id/scan/status", handler.GetLibraryScanStatus)
		libraryGroup.POST("/:id/metadata/refresh", handler.RefreshMetadata)
		libraryGroup.GET("/:id/stats", handler.GetLibraryStats)
	}
	
	// Note: Admin endpoints for library management are handled by the admin module
	// to avoid route conflicts. This module focuses on media operations.
}