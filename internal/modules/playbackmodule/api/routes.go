// Package api provides HTTP API endpoints for the playback module.
// Routes registers all playback-related endpoints with the Gin router.
package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all playback module routes with the given router group.
// It sets up endpoints for playback decisions, session management, and streaming.
//
// Endpoints:
//   - POST /decide - Determine playback method for a media file
//   - POST /session/start - Start a new playback session
//   - PUT /session/:id - Update session position/status
//   - DELETE /session/:id - End a playback session
//   - POST /session/:id/heartbeat - Keep session alive
//   - GET /stream/direct/:sessionId/*filepath - Stream file with range support
//   - POST /stream/prepare - Prepare streaming URL for a media file
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	// Playback decision endpoint
	router.POST("/decide", handler.DecidePlayback)

	// Compatibility check endpoint
	router.POST("/compatibility", handler.GetPlaybackCompatibility)

	// Session management endpoints
	session := router.Group("/session")
	{
		session.POST("/start", handler.StartSession)
		session.PUT("/:id", handler.UpdateSession)
		session.DELETE("/:id", handler.EndSession)
		session.POST("/:id/heartbeat", handler.Heartbeat)
	}

	// Streaming endpoints
	stream := router.Group("/stream")
	{
		// Direct file streaming with range support
		stream.GET("/direct/:sessionId/*filepath", handler.StreamDirect)

		// Prepare streaming URL
		stream.POST("/prepare", handler.PrepareStream)
	}
}
