package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterSubtitleRoutes registers subtitle-related routes.
func RegisterSubtitleRoutes(rg *gin.RouterGroup, handler *handlers.SubtitleHandler) {
	if handler == nil {
		return
	}

	// Subtitles are nested under media routes
	media := rg.Group("/media")

	// Get subtitle by track ID (converts to WebVTT)
	media.GET("/:id/subtitles/:trackId", handler.GetSubtitle)

	// Get subtitle by stream index (for embedded subtitles) - blocking, waits for full extraction
	media.GET("/:id/subtitles/stream/:index", handler.GetSubtitleByStreamIndex)

	// Text subtitle streaming: fast demux + SRT-to-WebVTT conversion on-the-fly
	media.GET("/:id/subtitles/text/:index/stream", handler.StreamTextSubtitle)

	// PGS/bitmap subtitle streaming: renders to WebP images for client-side overlay
	// Returns all frames as JSON array (for caching/preloading)
	media.GET("/:id/subtitles/pgs/:index", handler.GetAllPGSFrames)
	// Returns frames as streaming JSON lines (for progressive loading)
	media.GET("/:id/subtitles/pgs/:index/stream", handler.StreamPGSSubtitle)
}
