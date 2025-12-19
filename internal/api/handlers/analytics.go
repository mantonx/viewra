package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/analytics"
	transcodeanalytics "github.com/mantonx/viewra/internal/application/transcode_analytics"
)

// AnalyticsHandler handles playback analytics requests.
type AnalyticsHandler struct {
	service           *analytics.Service
	transcodeAnalytics *transcodeanalytics.Service
}

// NewAnalyticsHandler creates a new analytics handler.
func NewAnalyticsHandler(service *analytics.Service, transcodeAnalytics *transcodeanalytics.Service) *AnalyticsHandler {
	return &AnalyticsHandler{
		service:           service,
		transcodeAnalytics: transcodeAnalytics,
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
	StartupTimeMs      *int64 `json:"startupTimeMs"`
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
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert HTTP request to application request
	var session *analytics.PlaybackSession
	if req.Session != nil {
		session = &analytics.PlaybackSession{
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
			StartupTimeMs:      req.Session.StartupTimeMs,
		}
	}

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

	err := h.service.RecordPlayback(c.Request.Context(), &analytics.RecordPlaybackRequest{
		Session: session,
		Events:  events,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal server error",
			Message: "failed to store analytics data",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PlaybackSessionResponse represents a playback session in API responses.
type PlaybackSessionResponse struct {
	SessionID          string `json:"sessionId"`
	MediaID            int64  `json:"mediaId"`
	StartTime          int64  `json:"startTime"`
	EndTime            *int64 `json:"endTime,omitempty"`
	TotalPlayTimeMs    int64  `json:"totalPlayTimeMs"`
	TotalBufferTimeMs  int64  `json:"totalBufferTimeMs"`
	StallCount         int    `json:"stallCount"`
	QualitySwitchCount int    `json:"qualitySwitchCount"`
	AverageQuality     string `json:"averageQuality,omitempty"`
	DeviceType         string `json:"deviceType,omitempty"`
	ConnectionType     string `json:"connectionType,omitempty"`
	StartupTimeMs      *int64 `json:"startupTimeMs,omitempty"`

	// Transcode metrics (from backend, joined via session_id)
	QualityProfile   *string `json:"qualityProfile,omitempty"`
	TranscodeStrategy *string `json:"transcodeStrategy,omitempty"`
	HWAccel          *string `json:"hwAccel,omitempty"`
	BackendStartupMs *int64  `json:"backendStartupMs,omitempty"`
	FirstFrameMs     *int64  `json:"firstFrameMs,omitempty"`
	FirstSegmentMs   *int64  `json:"firstSegmentMs,omitempty"`
	TranscodeStatus  *string `json:"transcodeStatus,omitempty"`
	SegmentsCreated  *int64  `json:"segmentsCreated,omitempty"`
}

// PlaybackSummaryResponse represents aggregated analytics in API responses.
type PlaybackSummaryResponse struct {
	TotalSessions       int64   `json:"totalSessions"`
	UniqueMedia         int64   `json:"uniqueMedia,omitempty"`
	AvgPlayTimeMs       float64 `json:"avgPlayTimeMs"`
	AvgBufferTimeMs     float64 `json:"avgBufferTimeMs"`
	TotalStalls         int64   `json:"totalStalls"`
	AvgStallsPerSession float64 `json:"avgStallsPerSession"`
	AvgStartupTimeMs    float64 `json:"avgStartupTimeMs"`
	MinStartupTimeMs    *int64  `json:"minStartupTimeMs,omitempty"`
	MaxStartupTimeMs    *int64  `json:"maxStartupTimeMs,omitempty"`
}

// ListSessionsResponse represents the response for listing sessions.
type ListSessionsResponse struct {
	Sessions []PlaybackSessionResponse `json:"sessions"`
	Total    int                       `json:"total"`
}

// GetSession handles GET /api/analytics/sessions/:id
// @Summary Get playback session by ID
// @Description Retrieves a single playback session by its ID
// @Tags analytics
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} PlaybackSessionResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/analytics/sessions/{id} [get]
func (h *AnalyticsHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: "session ID is required",
		})
		return
	}

	session, err := h.service.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal server error",
			Message: "failed to retrieve session",
		})
		return
	}

	if session == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not found",
			Message: "session not found",
		})
		return
	}

	c.JSON(http.StatusOK, PlaybackSessionResponse{
		SessionID:          session.SessionID,
		MediaID:            session.MediaID,
		StartTime:          session.StartTime,
		EndTime:            session.EndTime,
		TotalPlayTimeMs:    session.TotalPlayTimeMs,
		TotalBufferTimeMs:  session.TotalBufferTimeMs,
		StallCount:         session.StallCount,
		QualitySwitchCount: session.QualitySwitchCount,
		AverageQuality:     session.AverageQuality,
		DeviceType:         session.DeviceType,
		ConnectionType:     session.ConnectionType,
		StartupTimeMs:      session.StartupTimeMs,
	})
}

// ListSessions handles GET /api/analytics/sessions
// @Summary List playback sessions
// @Description Lists playback sessions, optionally filtered by media ID
// @Tags analytics
// @Produce json
// @Param media_id query int false "Filter by media ID"
// @Param limit query int false "Maximum number of results (default 50)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Success 200 {object} ListSessionsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/analytics/sessions [get]
func (h *AnalyticsHandler) ListSessions(c *gin.Context) {
	mediaIDStr := c.Query("media_id")
	if mediaIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: "media_id is required",
		})
		return
	}

	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: "invalid media_id",
		})
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	ctx := c.Request.Context()

	// Use correlated query if transcode analytics is available (includes backend metrics)
	if h.transcodeAnalytics != nil {
		correlated, err := h.transcodeAnalytics.GetCorrelated(ctx, mediaID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "internal server error",
				Message: "failed to retrieve sessions",
			})
			return
		}

		response := make([]PlaybackSessionResponse, len(correlated))
		for i, r := range correlated {
			resp := PlaybackSessionResponse{
				SessionID:        r.SessionID,
				MediaID:          r.MediaID,
				StartupTimeMs:    r.FrontendStartupMs,
				QualityProfile:   r.QualityProfile,
				TranscodeStrategy: r.Strategy,
				HWAccel:          r.HWAccel,
				BackendStartupMs: r.BackendStartupMs,
				FirstFrameMs:     r.FirstFrameMs,
				FirstSegmentMs:   r.FirstSegmentMs,
				TranscodeStatus:  r.TranscodeStatus,
				SegmentsCreated:  r.SegmentsCreated,
			}
			if r.TotalPlayTimeMs != nil {
				resp.TotalPlayTimeMs = *r.TotalPlayTimeMs
			}
			if r.TotalBufferTimeMs != nil {
				resp.TotalBufferTimeMs = *r.TotalBufferTimeMs
			}
			if r.StallCount != nil {
				resp.StallCount = int(*r.StallCount)
			}
			response[i] = resp
		}

		c.JSON(http.StatusOK, ListSessionsResponse{
			Sessions: response,
			Total:    len(response),
		})
		return
	}

	// Fallback to basic sessions without transcode data
	sessions, err := h.service.ListSessions(ctx, mediaID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal server error",
			Message: "failed to retrieve sessions",
		})
		return
	}

	response := make([]PlaybackSessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = PlaybackSessionResponse{
			SessionID:          s.SessionID,
			MediaID:            s.MediaID,
			StartTime:          s.StartTime,
			EndTime:            s.EndTime,
			TotalPlayTimeMs:    s.TotalPlayTimeMs,
			TotalBufferTimeMs:  s.TotalBufferTimeMs,
			StallCount:         s.StallCount,
			QualitySwitchCount: s.QualitySwitchCount,
			AverageQuality:     s.AverageQuality,
			DeviceType:         s.DeviceType,
			ConnectionType:     s.ConnectionType,
			StartupTimeMs:      s.StartupTimeMs,
		}
	}

	c.JSON(http.StatusOK, ListSessionsResponse{
		Sessions: response,
		Total:    len(response),
	})
}

// GetMediaSummary handles GET /api/analytics/summary
// @Summary Get playback analytics summary
// @Description Retrieves aggregated playback analytics, optionally filtered by media ID
// @Tags analytics
// @Produce json
// @Param media_id query int false "Filter by media ID (omit for overall summary)"
// @Success 200 {object} PlaybackSummaryResponse
// @Router /api/analytics/summary [get]
func (h *AnalyticsHandler) GetMediaSummary(c *gin.Context) {
	mediaIDStr := c.Query("media_id")

	var summary *analytics.PlaybackSummary
	var err error

	if mediaIDStr != "" {
		mediaID, parseErr := strconv.ParseInt(mediaIDStr, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid request",
				Message: "invalid media_id",
			})
			return
		}
		summary, err = h.service.GetMediaSummary(c.Request.Context(), mediaID)
	} else {
		summary, err = h.service.GetOverallSummary(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal server error",
			Message: "failed to retrieve summary",
		})
		return
	}

	c.JSON(http.StatusOK, PlaybackSummaryResponse{
		TotalSessions:       summary.TotalSessions,
		UniqueMedia:         summary.UniqueMedia,
		AvgPlayTimeMs:       summary.AvgPlayTimeMs,
		AvgBufferTimeMs:     summary.AvgBufferTimeMs,
		TotalStalls:         summary.TotalStalls,
		AvgStallsPerSession: summary.AvgStallsPerSession,
		AvgStartupTimeMs:    summary.AvgStartupTimeMs,
		MinStartupTimeMs:    summary.MinStartupTimeMs,
		MaxStartupTimeMs:    summary.MaxStartupTimeMs,
	})
}
