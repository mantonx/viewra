package analytics

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository handles playback analytics persistence.
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

// PlaybackSession represents a playback session.
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

// QualitySwitchEvent represents a quality switch event.
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

// UpsertSession creates or updates a playback session.
func (r *Repository) UpsertSession(ctx context.Context, session *PlaybackSession) error {
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
	})
	return err
}

// CreateEvent creates a new quality switch event.
func (r *Repository) CreateEvent(ctx context.Context, event *QualitySwitchEvent) error {
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
func (r *Repository) CreateEvents(ctx context.Context, events []QualitySwitchEvent) error {
	for _, event := range events {
		if err := r.CreateEvent(ctx, &event); err != nil {
			return err
		}
	}
	return nil
}
