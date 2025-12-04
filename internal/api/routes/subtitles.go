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

	// Note: PGS/bitmap subtitles are handled via burn-in during transcode.
	// The standalone extraction endpoint was removed because it requires scanning
	// the entire video file (too slow for interactive use).
	// For HLS subtitle support, see /api/media/:id/hls/subtitle/:trackIndex/subtitles.vtt
}
