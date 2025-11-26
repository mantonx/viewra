package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/analytics"
)

// AnalyticsHandler handles playback analytics requests.
type AnalyticsHandler struct {
	repo   *analytics.Repository
	logger *slog.Logger
}

// NewAnalyticsHandler creates a new analytics handler.
func NewAnalyticsHandler(repo *analytics.Repository, logger *slog.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		repo:   repo,
		logger: logger,
	}
}

// QualitySwitchEventRequest represents a quality switch event from the client.
type QualitySwitchEventRequest struct {
	MediaID          int64    `json:"mediaId"`
	SessionID        string   `json:"sessionId"`
	FromQuality      *string  `json:"fromQuality"`
	ToQuality        string   `json:"toQuality"`
	SwitchReason     string   `json:"switchReason"`
	PositionSeconds  float64  `json:"positionSeconds"`
	NetworkSpeedMbps *float64 `json:"networkSpeedMbps"`
	BufferSeconds    *float64 `json:"bufferSeconds"`
	CausedStall      bool     `json:"causedStall"`
	DeviceType       string   `json:"deviceType"`
	ConnectionType   string   `json:"connectionType"`
	Timestamp        int64    `json:"timestamp"`
}

// PlaybackSessionRequest represents a playback session from the client.
type PlaybackSessionRequest struct {
	SessionID          string `json:"sessionId"`
	MediaID            int64  `json:"mediaId"`
	StartTime          int64  `json:"startTime"`
	EndTime            *int64 `json:"endTime"`
	TotalPlayTime      int64  `json:"totalPlayTime"`
	TotalBufferTime    int64  `json:"totalBufferTime"`
	StallCount         int    `json:"stallCount"`
	QualitySwitchCount int    `json:"qualitySwitchCount"`
	AverageQuality     string `json:"averageQuality"`
	DeviceType         string `json:"deviceType"`
	ConnectionType     string `json:"connectionType"`
}

// PlaybackAnalyticsRequest represents a batch of analytics data.
type PlaybackAnalyticsRequest struct {
	Session *PlaybackSessionRequest     `json:"session"`
	Events  []QualitySwitchEventRequest `json:"events"`
}

// RecordPlaybackAnalytics handles POST /api/analytics/playback
// @Summary Record playback analytics
// @Description Stores quality switch events and session data for analytics
// @Tags analytics
// @Accept json
// @Produce json
// @Param request body PlaybackAnalyticsRequest true "Analytics data"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} ErrorResponse
// @Router /api/analytics/playback [post]
func (h *AnalyticsHandler) RecordPlaybackAnalytics(c *gin.Context) {
	var req PlaybackAnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid analytics request", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Upsert session if provided
	if req.Session != nil {
		session := &analytics.PlaybackSession{
			SessionID:          req.Session.SessionID,
			MediaID:            req.Session.MediaID,
			StartTime:          req.Session.StartTime,
			EndTime:            req.Session.EndTime,
			TotalPlayTimeMs:    req.Session.TotalPlayTime,
			TotalBufferTimeMs:  req.Session.TotalBufferTime,
			StallCount:         req.Session.StallCount,
			QualitySwitchCount: req.Session.QualitySwitchCount,
			AverageQuality:     req.Session.AverageQuality,
			DeviceType:         req.Session.DeviceType,
			ConnectionType:     req.Session.ConnectionType,
		}
		if err := h.repo.UpsertSession(ctx, session); err != nil {
			h.logger.Error("failed to upsert session", "error", err, "sessionId", req.Session.SessionID)
			// Continue with events even if session fails
		}
	}

	// Insert events
	if len(req.Events) > 0 {
		events := make([]analytics.QualitySwitchEvent, len(req.Events))
		for i, e := range req.Events {
			events[i] = analytics.QualitySwitchEvent{
				SessionID:        e.SessionID,
				MediaID:          e.MediaID,
				FromQuality:      e.FromQuality,
				ToQuality:        e.ToQuality,
				SwitchReason:     e.SwitchReason,
				PositionSeconds:  e.PositionSeconds,
				NetworkSpeedMbps: e.NetworkSpeedMbps,
				BufferSeconds:    e.BufferSeconds,
				CausedStall:      e.CausedStall,
				DeviceType:       e.DeviceType,
				ConnectionType:   e.ConnectionType,
				Timestamp:        e.Timestamp,
			}
		}

		if err := h.repo.CreateEvents(ctx, events); err != nil {
			h.logger.Error("failed to insert events", "error", err, "count", len(req.Events))
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "internal server error",
				Message: "failed to store analytics events",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
