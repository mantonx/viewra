package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// GetSession handles GET /api/v1/transcoding/sessions/:id
//
// Returns detailed information about a specific transcoding session.
//
// URL parameters:
//   - id: Session ID
//
// Response:
//
//	{
//	  "sessionId": "string",
//	  "mediaId": "string",
//	  "provider": "string",
//	  "status": "string",
//	  "progress": 0.0,
//	  "startTime": "2024-01-01T00:00:00Z",
//	  "contentHash": "string",
//	  "error": "string"
//	}
func (h *APIHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}

	session, err := h.service.GetSession(sessionID)
	if err != nil {
		logger.Error("Failed to get session: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
		})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetProgress handles GET /api/v1/transcoding/sessions/:id/progress
//
// Returns real-time progress information for an active transcoding session.
//
// URL parameters:
//   - id: Session ID
//
// Response:
//
//	{
//	  "sessionId": "string",
//	  "percentComplete": 75.5,
//	  "currentBitrate": 2500000,
//	  "timeElapsed": 120.5,
//	  "estimatedTimeRemaining": 45.2,
//	  "status": "running"
//	}
func (h *APIHandler) GetProgress(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}

	progress, err := h.service.GetProgress(sessionID)
	if err != nil {
		logger.Error("Failed to get progress: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found or no progress available",
		})
		return
	}

	c.JSON(http.StatusOK, progress)
}

// StopTranscode handles DELETE /api/v1/transcoding/sessions/:id
//
// Stops an active transcoding session.
//
// URL parameters:
//   - id: Session ID to stop
//
// Response:
//
//	{
//	  "message": "Transcoding stopped successfully",
//	  "sessionId": "string"
//	}
func (h *APIHandler) StopTranscode(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}

	err := h.service.StopTranscode(sessionID)
	if err != nil {
		logger.Error("Failed to stop transcode: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to stop transcoding",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transcoding stopped successfully",
		"sessionId": sessionID,
	})
}

// ListSessions handles GET /api/v1/transcoding/sessions
//
// Returns a list of all transcoding sessions.
//
// Query parameters:
//   - status: Filter by status (optional)
//   - limit: Maximum number of results (default: 100)
//   - offset: Pagination offset (default: 0)
//
// Response:
//
//	{
//	  "sessions": [...],
//	  "total": 42,
//	  "limit": 100,
//	  "offset": 0
//	}
func (h *APIHandler) ListSessions(c *gin.Context) {
	// For now, return all sessions
	// TODO: Implement filtering and pagination
	sessions := h.service.GetAllSessions()

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
		"limit":    100,
		"offset":   0,
	})
}