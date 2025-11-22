package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterImageRoutes registers all image-related routes
func RegisterImageRoutes(r *gin.Engine, h *handlers.ImagesHandler) {
	// Image metadata and serving
	images := r.Group("/api/images")
	{
		images.GET("/:id", h.GetImage)           // Get image metadata
		images.GET("/:id/file", h.ServeImage)    // Serve actual image file
		images.POST("/batch", h.GetBatchMediaImages) // Batch get images
	}

	// Media-specific image endpoints
	r.GET("/api/media/:id/images", h.GetMediaImages)
	r.GET("/api/movies/:id/images", h.GetMovieImages)
	r.GET("/api/tv/episodes/:id/images", h.GetEpisodeImages)

	// Entity-specific image endpoints (TV shows, seasons, music albums, music artists)
	r.GET("/api/tv/shows/:id/images", h.GetTVShowImages)
	r.GET("/api/tv/seasons/:id/images", h.GetTVSeasonImages)
	r.GET("/api/music/albums/:id/images", h.GetMusicAlbumImages)
	r.GET("/api/music/artists/:id/images", h.GetMusicArtistImages)
}
