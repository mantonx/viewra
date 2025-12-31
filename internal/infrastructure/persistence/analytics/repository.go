package analytics

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/application/analytics"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository handles playback analytics persistence.
// Implements analytics.Repository interface.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new analytics repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// UpsertSession creates or updates a playback session.
func (r *Repository) UpsertSession(ctx context.Context, session *analytics.PlaybackSession) error {
	_, err := r.Q().UpsertPlaybackSession(ctx, unified.UpsertPlaybackSessionParams{
		SessionID:          session.SessionID,
		MediaID:            session.MediaID,
		StartTime:          session.StartTime,
		EndTime:            common.NullInt64Ptr(session.EndTime),
		TotalPlayTimeMs:    common.NullInt64(session.TotalPlayTimeMs),
		TotalBufferTimeMs:  common.NullInt64(session.TotalBufferTimeMs),
		StallCount:         common.NullInt64(int64(session.StallCount)),
		QualitySwitchCount: common.NullInt64(int64(session.QualitySwitchCount)),
		AverageQuality:     common.NullString(session.AverageQuality),
		DeviceType:         common.NullString(session.DeviceType),
		ConnectionType:     common.NullString(session.ConnectionType),
		StartupTimeMs:      common.NullInt64Ptr(session.StartupTimeMs),
	})
	return err
}

// CreateEvent creates a new quality switch event.
func (r *Repository) CreateEvent(ctx context.Context, event *analytics.QualitySwitchEvent) error {
	causedStall := int64(0)
	if event.CausedStall {
		causedStall = 1
	}

	_, err := r.Q().CreateQualitySwitchEvent(ctx, unified.CreateQualitySwitchEventParams{
		SessionID:        event.SessionID,
		MediaID:          event.MediaID,
		FromQuality:      common.NullStringPtr(event.FromQuality),
		ToQuality:        event.ToQuality,
		SwitchReason:     event.SwitchReason,
		PositionSeconds:  event.PositionSeconds,
		NetworkSpeedMbps: common.NullFloat64Ptr(event.NetworkSpeedMbps),
		BufferSeconds:    common.NullFloat64Ptr(event.BufferSeconds),
		CausedStall:      common.NullInt64(causedStall),
		DeviceType:       common.NullString(event.DeviceType),
		ConnectionType:   common.NullString(event.ConnectionType),
		Timestamp:        event.Timestamp,
	})
	return err
}

// CreateEvents creates multiple quality switch events in a batch.
func (r *Repository) CreateEvents(ctx context.Context, events []analytics.QualitySwitchEvent) error {
	for _, event := range events {
		if err := r.CreateEvent(ctx, &event); err != nil {
			return err
		}
	}
	return nil
}

// GetSessionByID retrieves a playback session by its ID.
func (r *Repository) GetSessionByID(ctx context.Context, sessionID string) (*analytics.PlaybackSession, error) {
	row, err := r.Q().GetPlaybackSessionByID(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return sessionToDomain(row), nil
}

// ListSessionsByMediaID retrieves playback sessions for a specific media item.
func (r *Repository) ListSessionsByMediaID(ctx context.Context, mediaID int64, limit, offset int) ([]analytics.PlaybackSession, error) {
	rows, err := r.Q().ListPlaybackSessionsByMediaID(ctx, unified.ListPlaybackSessionsByMediaIDParams{
		MediaID: mediaID,
		Limit:   int64(limit),
		Offset:  int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, func(row unified.PlaybackSession) analytics.PlaybackSession {
		return *sessionToDomain(row)
	}), nil
}

// GetSummaryByMediaID retrieves aggregated analytics for a specific media item.
func (r *Repository) GetSummaryByMediaID(ctx context.Context, mediaID int64) (*analytics.PlaybackSummary, error) {
	row, err := r.Q().GetPlaybackSummaryByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	return &analytics.PlaybackSummary{
		TotalSessions:       row.TotalSessions,
		AvgPlayTimeMs:       common.Float64FromInterface(row.AvgPlayTimeMs),
		AvgBufferTimeMs:     common.Float64FromInterface(row.AvgBufferTimeMs),
		TotalStalls:         common.Int64FromInterface(row.TotalStalls),
		AvgStallsPerSession: common.Float64FromInterface(row.AvgStallsPerSession),
		AvgStartupTimeMs:    common.Float64FromInterface(row.AvgStartupTimeMs),
		MinStartupTimeMs:    common.Int64PtrFromInterface(row.MinStartupTimeMs),
		MaxStartupTimeMs:    common.Int64PtrFromInterface(row.MaxStartupTimeMs),
	}, nil
}

// GetOverallSummary retrieves aggregated analytics across all media.
func (r *Repository) GetOverallSummary(ctx context.Context) (*analytics.PlaybackSummary, error) {
	row, err := r.Q().GetOverallPlaybackSummary(ctx)
	if err != nil {
		return nil, err
	}
	return &analytics.PlaybackSummary{
		TotalSessions:       row.TotalSessions,
		UniqueMedia:         row.UniqueMedia,
		AvgPlayTimeMs:       common.Float64FromInterface(row.AvgPlayTimeMs),
		AvgBufferTimeMs:     common.Float64FromInterface(row.AvgBufferTimeMs),
		TotalStalls:         common.Int64FromInterface(row.TotalStalls),
		AvgStallsPerSession: common.Float64FromInterface(row.AvgStallsPerSession),
		AvgStartupTimeMs:    common.Float64FromInterface(row.AvgStartupTimeMs),
		MinStartupTimeMs:    common.Int64PtrFromInterface(row.MinStartupTimeMs),
		MaxStartupTimeMs:    common.Int64PtrFromInterface(row.MaxStartupTimeMs),
	}, nil
}

// sessionToDomain converts a database row to domain model.
func sessionToDomain(row unified.PlaybackSession) *analytics.PlaybackSession {
	return &analytics.PlaybackSession{
		SessionID:          row.SessionID,
		MediaID:            row.MediaID,
		StartTime:          row.StartTime,
		EndTime:            common.ParseNullInt64Ptr(row.EndTime),
		TotalPlayTimeMs:    common.ParseNullInt64(row.TotalPlayTimeMs),
		TotalBufferTimeMs:  common.ParseNullInt64(row.TotalBufferTimeMs),
		StallCount:         int(common.ParseNullInt64(row.StallCount)),
		QualitySwitchCount: int(common.ParseNullInt64(row.QualitySwitchCount)),
		AverageQuality:     common.ParseNullString(row.AverageQuality),
		DeviceType:         common.ParseNullString(row.DeviceType),
		ConnectionType:     common.ParseNullString(row.ConnectionType),
		StartupTimeMs:      common.ParseNullInt64Ptr(row.StartupTimeMs),
	}
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
