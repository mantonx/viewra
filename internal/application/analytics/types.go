package analytics

import "context"

// PlaybackSession represents a playback session for analytics
type PlaybackSession struct {
	SessionID          string
	MediaID            int64
	StartTime          int64
	EndTime            *int64
	TotalPlayTimeMs    int64
	TotalBufferTimeMs  int64
	StallCount         int
	QualitySwitchCount int
	AverageQuality     string
	DeviceType         string
	ConnectionType     string
	StartupTimeMs      *int64
}

// QualitySwitchEvent represents a quality switch event
type QualitySwitchEvent struct {
	SessionID        string
	MediaID          int64
	FromQuality      *string
	ToQuality        string
	SwitchReason     string
	PositionSeconds  float64
	NetworkSpeedMbps *float64
	BufferSeconds    *float64
	CausedStall      bool
	DeviceType       string
	ConnectionType   string
	Timestamp        int64
}

// PlaybackSummary contains aggregated analytics for playback sessions
type PlaybackSummary struct {
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

// Repository defines the interface for analytics persistence
type Repository interface {
	UpsertSession(ctx context.Context, session *PlaybackSession) error
	CreateEvents(ctx context.Context, events []QualitySwitchEvent) error
	GetSessionByID(ctx context.Context, sessionID string) (*PlaybackSession, error)
	ListSessionsByMediaID(ctx context.Context, mediaID int64, limit, offset int) ([]PlaybackSession, error)
	GetSummaryByMediaID(ctx context.Context, mediaID int64) (*PlaybackSummary, error)
	GetOverallSummary(ctx context.Context) (*PlaybackSummary, error)
}
