package analytics

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/application/analytics"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository handles playback analytics persistence.
// Implements analytics.Repository interface.
type Repository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewRepository creates a new analytics repository.
func NewRepository(db *sql.DB, dbType string) *Repository {
	r := &Repository{
		db:     db,
		dbType: dbType,
	}

	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}

	return r
}

// UpsertSession creates or updates a playback session.
func (r *Repository) UpsertSession(ctx context.Context, session *analytics.PlaybackSession) error {
	if r.dbType == "sqlite" {
		_, err := r.sqliteQuerier.UpsertPlaybackSession(ctx, sqlc_sqlite.UpsertPlaybackSessionParams{
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

	// PostgreSQL
	_, err := r.postgresQuerier.UpsertPlaybackSession(ctx, sqlc_postgres.UpsertPlaybackSessionParams{
		SessionID:          session.SessionID,
		MediaID:            int32(session.MediaID),
		StartTime:          session.StartTime,
		EndTime:            common.NullInt64Ptr(session.EndTime),
		TotalPlayTimeMs:    common.NullInt32FromInt64(session.TotalPlayTimeMs),
		TotalBufferTimeMs:  common.NullInt32FromInt64(session.TotalBufferTimeMs),
		StallCount:         common.NullInt32FromInt64(int64(session.StallCount)),
		QualitySwitchCount: common.NullInt32FromInt64(int64(session.QualitySwitchCount)),
		AverageQuality:     common.NullString(session.AverageQuality),
		DeviceType:         common.NullString(session.DeviceType),
		ConnectionType:     common.NullString(session.ConnectionType),
		StartupTimeMs:      common.NullInt32Ptr(session.StartupTimeMs),
	})
	return err
}

// CreateEvent creates a new quality switch event.
func (r *Repository) CreateEvent(ctx context.Context, event *analytics.QualitySwitchEvent) error {
	causedStall := int64(0)
	if event.CausedStall {
		causedStall = 1
	}

	if r.dbType == "sqlite" {
		_, err := r.sqliteQuerier.CreateQualitySwitchEvent(ctx, sqlc_sqlite.CreateQualitySwitchEventParams{
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

	// PostgreSQL
	_, err := r.postgresQuerier.CreateQualitySwitchEvent(ctx, sqlc_postgres.CreateQualitySwitchEventParams{
		SessionID:        event.SessionID,
		MediaID:          int32(event.MediaID),
		FromQuality:      common.NullStringPtr(event.FromQuality),
		ToQuality:        event.ToQuality,
		SwitchReason:     event.SwitchReason,
		PositionSeconds:  float32(event.PositionSeconds),
		NetworkSpeedMbps: common.NullFloat32Ptr(event.NetworkSpeedMbps),
		BufferSeconds:    common.NullFloat32Ptr(event.BufferSeconds),
		CausedStall:      common.NullBool(event.CausedStall),
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
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetPlaybackSessionByID(ctx, sessionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
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
		}, nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetPlaybackSessionByID(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &analytics.PlaybackSession{
		SessionID:          row.SessionID,
		MediaID:            int64(row.MediaID),
		StartTime:          row.StartTime,
		EndTime:            common.ParseNullInt64Ptr(row.EndTime),
		TotalPlayTimeMs:    common.ParseNullInt32(row.TotalPlayTimeMs),
		TotalBufferTimeMs:  common.ParseNullInt32(row.TotalBufferTimeMs),
		StallCount:         int(common.ParseNullInt32(row.StallCount)),
		QualitySwitchCount: int(common.ParseNullInt32(row.QualitySwitchCount)),
		AverageQuality:     common.ParseNullString(row.AverageQuality),
		DeviceType:         common.ParseNullString(row.DeviceType),
		ConnectionType:     common.ParseNullString(row.ConnectionType),
		StartupTimeMs:      common.ParseNullInt32Ptr(row.StartupTimeMs),
	}, nil
}

// ListSessionsByMediaID retrieves playback sessions for a specific media item.
func (r *Repository) ListSessionsByMediaID(ctx context.Context, mediaID int64, limit, offset int) ([]analytics.PlaybackSession, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.ListPlaybackSessionsByMediaID(ctx, sqlc_sqlite.ListPlaybackSessionsByMediaIDParams{
			MediaID: mediaID,
			Limit:   int64(limit),
			Offset:  int64(offset),
		})
		if err != nil {
			return nil, err
		}

		sessions := make([]analytics.PlaybackSession, len(rows))
		for i, row := range rows {
			sessions[i] = analytics.PlaybackSession{
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
		return sessions, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.ListPlaybackSessionsByMediaID(ctx, sqlc_postgres.ListPlaybackSessionsByMediaIDParams{
		MediaID: int32(mediaID),
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}

	sessions := make([]analytics.PlaybackSession, len(rows))
	for i, row := range rows {
		sessions[i] = analytics.PlaybackSession{
			SessionID:          row.SessionID,
			MediaID:            int64(row.MediaID),
			StartTime:          row.StartTime,
			EndTime:            common.ParseNullInt64Ptr(row.EndTime),
			TotalPlayTimeMs:    common.ParseNullInt32(row.TotalPlayTimeMs),
			TotalBufferTimeMs:  common.ParseNullInt32(row.TotalBufferTimeMs),
			StallCount:         int(common.ParseNullInt32(row.StallCount)),
			QualitySwitchCount: int(common.ParseNullInt32(row.QualitySwitchCount)),
			AverageQuality:     common.ParseNullString(row.AverageQuality),
			DeviceType:         common.ParseNullString(row.DeviceType),
			ConnectionType:     common.ParseNullString(row.ConnectionType),
			StartupTimeMs:      common.ParseNullInt32Ptr(row.StartupTimeMs),
		}
	}
	return sessions, nil
}

// GetSummaryByMediaID retrieves aggregated analytics for a specific media item.
func (r *Repository) GetSummaryByMediaID(ctx context.Context, mediaID int64) (*analytics.PlaybackSummary, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetPlaybackSummaryByMediaID(ctx, mediaID)
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

	// PostgreSQL
	row, err := r.postgresQuerier.GetPlaybackSummaryByMediaID(ctx, int32(mediaID))
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
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetOverallPlaybackSummary(ctx)
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

	// PostgreSQL
	row, err := r.postgresQuerier.GetOverallPlaybackSummary(ctx)
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
