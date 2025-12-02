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

// Repository defines the interface for analytics persistence
type Repository interface {
	UpsertSession(ctx context.Context, session *PlaybackSession) error
	CreateEvents(ctx context.Context, events []QualitySwitchEvent) error
}
