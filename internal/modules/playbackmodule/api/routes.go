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
//   - POST /compatibility - Check playback compatibility for media files
//   - POST /sessions - Start a new playback session
//   - PUT /sessions/:sessionId - Update session position/status
//   - DELETE /sessions/:sessionId - End a playback session
//   - GET /sessions/:sessionId - Get session information
//   - GET /sessions - Get all active sessions
//   - POST /sessions/:sessionId/heartbeat - Keep session alive
//   - GET /stream/:sessionId - Stream based on session (supports all methods)
//   - GET /stream/file/:fileId - Stream file by ID  
//   - GET /stream/direct - Direct stream with path parameter
//   - POST /stream/prepare - Prepare streaming URL for a media file
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	// Playback decision endpoint
	router.POST("/decide", handler.DecidePlayback)

	// Compatibility check endpoint
	router.POST("/compatibility", handler.GetPlaybackCompatibility)

	// Session management endpoints
	sessions := router.Group("/sessions")
	{
		sessions.POST("", handler.StartSession)
		sessions.GET("", handler.GetActiveSessions)
		sessions.GET("/:sessionId", handler.GetSession)
		sessions.PUT("/:sessionId", handler.UpdateSession)
		sessions.DELETE("/:sessionId", handler.EndSession)
		sessions.POST("/:sessionId/heartbeat", handler.Heartbeat)
	}

	// Transcode endpoints (proxy to transcoding service)
	router.POST("/transcode", handler.StartTranscodeSession)
	router.DELETE("/transcode/:sessionId", handler.StopTranscodeSession)

	// Streaming endpoints
	stream := router.Group("/stream")
	{
		// Session-based streaming (recommended)
		stream.GET("/:sessionId", handler.StreamSession)
		stream.HEAD("/:sessionId", handler.StreamSession)
		
		// File ID based streaming
		stream.GET("/file/:fileId", handler.StreamFile)
		stream.HEAD("/file/:fileId", handler.StreamFile)

		// Direct file streaming with path (legacy)
		stream.GET("/direct", handler.StreamDirect)
		stream.HEAD("/direct", handler.StreamDirect)

		// Prepare streaming URL
		stream.POST("/prepare", handler.PrepareStream)
	}
}
