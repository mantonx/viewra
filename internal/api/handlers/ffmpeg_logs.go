package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
)

// FFmpegLogsHandler handles FFmpeg log-related HTTP requests.
type FFmpegLogsHandler struct {
	sessionManager *transcoding.SessionManager
}

// NewFFmpegLogsHandler creates a new FFmpeg logs handler.
func NewFFmpegLogsHandler(sessionManager *transcoding.SessionManager) *FFmpegLogsHandler {
	return &FFmpegLogsHandler{
		sessionManager: sessionManager,
	}
}

// LogListResponse represents a list of logs for a media item.
type LogListResponse struct {
	MediaID int64                    `json:"media_id"`
	Logs    []transcoding.LogInfo    `json:"logs"`
	Count   int                      `json:"count"`
}

// LogContentResponse represents log content.
type LogContentResponse struct {
	SessionID string `json:"session_id"`
	MediaID   int64  `json:"media_id"`
	Content   string `json:"content"`
	IsActive  bool   `json:"is_active"`
}

// ActiveSessionsResponse represents active transcode sessions with logs.
type ActiveSessionsResponse struct {
	Sessions []ActiveSessionInfo `json:"sessions"`
	Count    int                 `json:"count"`
}

// ActiveSessionInfo represents info about an active session.
type ActiveSessionInfo struct {
	SessionID string `json:"session_id"`
	MediaID   int64  `json:"media_id"`
	Quality   string `json:"quality"`
	HasLog    bool   `json:"has_log"`
}

// ListLogs lists all FFmpeg logs for a media item.
//
// @Summary List FFmpeg logs for media
// @Description Lists all available FFmpeg transcode logs for a specific media item
// @Tags ffmpeg-logs
// @Produce json
// @Param media_id path int true "Media ID"
// @Success 200 {object} LogListResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/ffmpeg-logs [get]
func (h *FFmpegLogsHandler) ListLogs(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	logStore := h.sessionManager.GetLogStore()
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "FFmpeg logging not enabled"})
		return
	}

	logs, err := logStore.ListLogs(mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to list logs: %v", err)})
		return
	}

	c.JSON(http.StatusOK, LogListResponse{
		MediaID: mediaID,
		Logs:    logs,
		Count:   len(logs),
	})
}

// GetLog retrieves a specific FFmpeg log.
//
// @Summary Get FFmpeg log content
// @Description Retrieves the content of a specific FFmpeg transcode log
// @Tags ffmpeg-logs
// @Produce json
// @Param media_id path int true "Media ID"
// @Param session_id path string true "Session ID"
// @Param tail query int false "Only return last N lines"
// @Success 200 {object} LogContentResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/ffmpeg-logs/{session_id} [get]
func (h *FFmpegLogsHandler) GetLog(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Session ID required"})
		return
	}

	logStore := h.sessionManager.GetLogStore()
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "FFmpeg logging not enabled"})
		return
	}

	// Check if tail parameter is specified
	var content string
	if tailStr := c.Query("tail"); tailStr != "" {
		tailLines, err := strconv.Atoi(tailStr)
		if err != nil || tailLines <= 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tail parameter"})
			return
		}

		lines, err := logStore.ReadLogTail(sessionID, mediaID, tailLines)
		if err != nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: fmt.Sprintf("Log not found: %v", err)})
			return
		}

		for _, line := range lines {
			content += line + "\n"
		}
	} else {
		var err error
		content, err = logStore.ReadLog(sessionID, mediaID)
		if err != nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: fmt.Sprintf("Log not found: %v", err)})
			return
		}
	}

	// Check if session is still active
	isActive := logStore.GetLogWriter(sessionID) != nil

	c.JSON(http.StatusOK, LogContentResponse{
		SessionID: sessionID,
		MediaID:   mediaID,
		Content:   content,
		IsActive:  isActive,
	})
}

// StreamLog streams FFmpeg log in real-time using Server-Sent Events.
//
// @Summary Stream FFmpeg log in real-time
// @Description Streams FFmpeg log output in real-time using Server-Sent Events (SSE).
// @Description For active sessions, new log lines are pushed as they appear.
// @Description For completed sessions, returns existing content then closes.
// @Tags ffmpeg-logs
// @Produce text/event-stream
// @Param media_id path int true "Media ID"
// @Param session_id path string true "Session ID"
// @Success 200 {string} string "SSE stream of log lines"
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/ffmpeg-logs/{session_id}/stream [get]
func (h *FFmpegLogsHandler) StreamLog(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Session ID required"})
		return
	}

	logStore := h.sessionManager.GetLogStore()
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "FFmpeg logging not enabled"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Get the writer and flusher
	w := c.Writer

	// Stream the log content
	ctx := c.Request.Context()
	err = logStore.StreamLog(ctx, sessionID, mediaID, true, &sseWriter{w: w, flusher: w})
	if err != nil && err != ctx.Err() {
		// Send error event
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		w.Flush()
	}

	// Send done event
	fmt.Fprintf(w, "event: done\ndata: stream ended\n\n")
	w.Flush()
}

// GetLogInfo retrieves detailed information about a specific log.
//
// @Summary Get FFmpeg log info
// @Description Retrieves metadata about a specific FFmpeg log (size, line count, error count)
// @Tags ffmpeg-logs
// @Produce json
// @Param media_id path int true "Media ID"
// @Param session_id path string true "Session ID"
// @Success 200 {object} transcoding.LogInfo
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/ffmpeg-logs/{session_id}/info [get]
func (h *FFmpegLogsHandler) GetLogInfo(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Session ID required"})
		return
	}

	logStore := h.sessionManager.GetLogStore()
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "FFmpeg logging not enabled"})
		return
	}

	info, err := logStore.GetLogInfo(sessionID, mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: fmt.Sprintf("Log not found: %v", err)})
		return
	}

	c.JSON(http.StatusOK, info)
}

// DeleteLog deletes a specific FFmpeg log.
//
// @Summary Delete FFmpeg log
// @Description Deletes a specific FFmpeg transcode log (cannot delete active logs)
// @Tags ffmpeg-logs
// @Produce json
// @Param media_id path int true "Media ID"
// @Param session_id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse "Cannot delete active log"
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/ffmpeg-logs/{session_id} [delete]
func (h *FFmpegLogsHandler) DeleteLog(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Session ID required"})
		return
	}

	logStore := h.sessionManager.GetLogStore()
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "FFmpeg logging not enabled"})
		return
	}

	err = logStore.DeleteLog(sessionID, mediaID)
	if err != nil {
		if err.Error() == "cannot delete active log" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: fmt.Sprintf("Failed to delete log: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Log deleted successfully"})
}

// ListActiveSessions lists all active transcode sessions with logging info.
//
// @Summary List active transcode sessions
// @Description Lists all currently active transcode sessions and whether they have logging enabled
// @Tags ffmpeg-logs
// @Produce json
// @Success 200 {object} ActiveSessionsResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/transcode/active-sessions [get]
func (h *FFmpegLogsHandler) ListActiveSessions(c *gin.Context) {
	logStore := h.sessionManager.GetLogStore()

	// Get active session IDs from log store
	var activeIDs []string
	if logStore != nil {
		activeIDs = logStore.GetActiveSessionIDs()
	}

	// Build response
	sessions := make([]ActiveSessionInfo, 0, len(activeIDs))
	for _, sessionID := range activeIDs {
		// Parse session ID to extract media ID and quality
		// Format: {mediaID}_{quality}_{timestamp}
		var mediaID int64
		var quality string
		var timestamp int64
		_, err := fmt.Sscanf(sessionID, "%d_%s_%d", &mediaID, &quality, &timestamp)
		if err != nil {
			// Try alternative parsing
			parts := splitSessionID(sessionID)
			if len(parts) >= 2 {
				mediaID, _ = strconv.ParseInt(parts[0], 10, 64)
				quality = parts[1]
			}
		}

		sessions = append(sessions, ActiveSessionInfo{
			SessionID: sessionID,
			MediaID:   mediaID,
			Quality:   quality,
			HasLog:    true,
		})
	}

	c.JSON(http.StatusOK, ActiveSessionsResponse{
		Sessions: sessions,
		Count:    len(sessions),
	})
}

// sseWriter wraps an http.ResponseWriter to format output as SSE events.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (s *sseWriter) Write(p []byte) (n int, err error) {
	// Write data as SSE event
	n, err = fmt.Fprintf(s.w, "data: %s\n\n", string(p))
	if err != nil {
		return n, err
	}
	s.flusher.Flush()
	return n, nil
}

// splitSessionID splits a session ID into its components.
func splitSessionID(sessionID string) []string {
	var parts []string
	var current string
	underscoreCount := 0

	for _, c := range sessionID {
		if c == '_' {
			underscoreCount++
			if underscoreCount <= 2 {
				parts = append(parts, current)
				current = ""
				continue
			}
		}
		current += string(c)
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
