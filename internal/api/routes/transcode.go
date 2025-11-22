package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
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

	// GET /api/media/:id/hls/:quality/playlist.m3u8 - Serve HLS playlist (with on-demand transcoding)
	router.GET("/media/:id/hls/:quality/playlist.m3u8", handler.ServePlaylist)

	// GET /api/media/:id/hls/:quality/:filename - Serve HLS segment files (MPEG-TS segments)
	router.GET("/media/:id/hls/:quality/:filename", handler.ServeHLSSegment)

	// GET /api/transcode/queue - Get queue statistics
	router.GET("/transcode/queue", handler.GetQueueStats)

	// POST /api/media/:id/transcode/:quality/cancel - Cancel transcode job
	router.POST("/media/:id/transcode/:quality/cancel", handler.CancelTranscodeJob)

	// GET /api/transcode/disk-usage - Get disk usage statistics
	router.GET("/transcode/disk-usage", handler.GetDiskUsage)

	// POST /api/transcode/cleanup - Cleanup transcode files
	router.POST("/transcode/cleanup", handler.CleanupTranscodes)
}
