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

	// Get subtitle by stream index (for embedded subtitles)
	media.GET("/:id/subtitles/stream/:index", handler.GetSubtitleByStreamIndex)
}
