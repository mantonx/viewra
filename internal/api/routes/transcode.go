package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterTranscodeRoutes registers all transcode-related routes
func RegisterTranscodeRoutes(router *gin.RouterGroup, handler *handlers.TranscodeHandler) {
	// Skip route registration if handler is nil
	if handler == nil {
		return
	}

	// POST /api/media/:id/transcode/:quality - Create transcode job
	router.POST("/media/:id/transcode/:quality", handler.CreateTranscodeJob)

	// GET /api/media/:id/transcode/:quality - Get transcode job status
	router.GET("/media/:id/transcode/:quality", handler.GetTranscodeStatus)

	// GET /api/media/:id/dash/:quality/manifest.mpd - Serve DASH manifest (with on-demand transcoding)
	router.GET("/media/:id/dash/:quality/manifest.mpd", handler.ServeManifest)

	// GET /api/media/:id/dash/:quality/segments/:segment - Serve DASH segment
	router.GET("/media/:id/dash/:quality/segments/:segment", handler.ServeDASHSegment)

	// GET /api/transcode/queue - Get queue statistics
	router.GET("/transcode/queue", handler.GetQueueStats)

	// POST /api/media/:id/transcode/:quality/cancel - Cancel transcode job
	router.POST("/media/:id/transcode/:quality/cancel", handler.CancelTranscodeJob)
}
