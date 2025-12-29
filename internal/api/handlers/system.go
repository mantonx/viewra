package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/app/lifecycle"
)

// SystemHandler handles system-level HTTP requests.
type SystemHandler struct {
	lifecycleMgr *lifecycle.Manager
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(lifecycleMgr *lifecycle.Manager) *SystemHandler {
	return &SystemHandler{lifecycleMgr: lifecycleMgr}
}

// RestartRequest is the request body for triggering a restart.
type RestartRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RestartResponse is the response for restart-related endpoints.
type RestartResponse struct {
	Pending         bool     `json:"pending"`
	Reason          string   `json:"reason,omitempty"`
	RequestedAt     string   `json:"requested_at,omitempty"`
	PendingSettings []string `json:"pending_settings,omitempty"`
	Message         string   `json:"message,omitempty"`
}

// RequestRestart handles POST /api/admin/system/restart
// @Summary Request server restart
// @Description Requests the server to restart (admin only). The restart is graceful.
// @Tags system
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body RestartRequest true "Restart reason"
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/restart [post]
func (h *SystemHandler) RequestRestart(c *gin.Context) {
	var req RestartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req.Reason = "Manual restart requested via API"
	}
	if req.Reason == "" {
		req.Reason = "Manual restart requested via API"
	}

	h.lifecycleMgr.RequestRestart(req.Reason)

	status := h.lifecycleMgr.GetRestartStatus()
	c.JSON(http.StatusOK, RestartResponse{
		Pending:         status.Pending,
		Reason:          status.Reason,
		RequestedAt:     formatTimePtr(status.RequestedAt),
		PendingSettings: status.PendingSettings,
		Message:         "Restart scheduled. Server will restart shortly.",
	})
}

// CancelRestart handles DELETE /api/admin/system/restart
// @Summary Cancel pending restart
// @Description Cancels a pending server restart (admin only)
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError "No pending restart"
// @Router /api/admin/system/restart [delete]
func (h *SystemHandler) CancelRestart(c *gin.Context) {
	if !h.lifecycleMgr.CancelRestart() {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "No pending restart to cancel",
		})
		return
	}

	c.JSON(http.StatusOK, RestartResponse{
		Pending: false,
		Message: "Restart cancelled",
	})
}

// GetRestartStatus handles GET /api/admin/system/restart
// @Summary Get restart status
// @Description Returns the current restart status including pending settings
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} RestartResponse
// @Failure 401 {object} APIError
// @Router /api/admin/system/restart [get]
func (h *SystemHandler) GetRestartStatus(c *gin.Context) {
	status := h.lifecycleMgr.GetRestartStatus()
	c.JSON(http.StatusOK, RestartResponse{
		Pending:         status.Pending,
		Reason:          status.Reason,
		RequestedAt:     formatTimePtr(status.RequestedAt),
		PendingSettings: status.PendingSettings,
	})
}

// ExecuteRestart handles POST /api/admin/system/restart/now
// @Summary Execute restart immediately
// @Description Triggers an immediate server restart (admin only). Use with caution.
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 202 {object} RestartResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/system/restart/now [post]
func (h *SystemHandler) ExecuteRestart(c *gin.Context) {
	// First ensure a restart is requested
	if !h.lifecycleMgr.HasPendingRestart() {
		h.lifecycleMgr.RequestRestart("Immediate restart requested via API")
	}

	c.JSON(http.StatusAccepted, RestartResponse{
		Pending: true,
		Message: "Restart initiated. Server is shutting down.",
	})

	// Execute restart in background after response is sent
	go func() {
		time.Sleep(500 * time.Millisecond) // Allow response to be sent
		h.lifecycleMgr.ExecuteRestart(context.Background())
	}()
}

// AdminStatusEvent represents an SSE event for admin status updates.
type AdminStatusEvent struct {
	Type            string   `json:"type"`
	RestartPending  bool     `json:"restart_pending"`
	PendingSettings []string `json:"pending_settings,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	Timestamp       string   `json:"timestamp"`
}

// StreamAdminStatus handles GET /api/admin/status/stream
// @Summary Stream admin status updates
// @Description SSE stream of admin-relevant status updates (restart pending, etc.)
// @Tags system
// @Security BearerAuth
// @Produce text/event-stream
// @Success 200 {object} AdminStatusEvent
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/admin/status/stream [get]
func (h *SystemHandler) StreamAdminStatus(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Send initial status
	status := h.lifecycleMgr.GetRestartStatus()
	event := AdminStatusEvent{
		Type:            "status",
		RestartPending:  status.Pending,
		PendingSettings: status.PendingSettings,
		Reason:          status.Reason,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeSSEEvent(c, "status", event); err != nil {
		return
	}
	c.Writer.Flush()

	// Create a ticker for periodic updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Stream updates
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.lifecycleMgr.ShutdownCh():
			// Server is shutting down
			event := AdminStatusEvent{
				Type:           "shutdown",
				RestartPending: true,
				Reason:         "Server is shutting down",
				Timestamp:      time.Now().UTC().Format(time.RFC3339),
			}
			_ = writeSSEEvent(c, "shutdown", event)
			c.Writer.Flush()
			return
		case <-ticker.C:
			// Send periodic status update
			status := h.lifecycleMgr.GetRestartStatus()
			event := AdminStatusEvent{
				Type:            "status",
				RestartPending:  status.Pending,
				PendingSettings: status.PendingSettings,
				Reason:          status.Reason,
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
			}
			if err := writeSSEEvent(c, "status", event); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

// writeSSEEvent writes an SSE event to the response.
func writeSSEEvent(c *gin.Context, eventType string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	return err
}

// formatTimePtr formats a time.Time, returning empty string for zero time.
func formatTimePtr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
