package transcodeanlytics

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// TranscodeAnalytics represents a transcode session's analytics data.
type TranscodeAnalytics struct {
	SessionID       string
	MediaID         int64
	QualityProfile  string
	Strategy        string
	HWAccel         string
	FFmpegStartMs   *int64
	FirstFrameMs    *int64
	FirstSegmentMs  *int64
	ManifestReadyMs *int64
	Status          string
	ErrorReason     string
	TotalDurationMs *int64
	SegmentsCreated *int64
	CreatedAt       int64
	CompletedAt     *int64
}

// TranscodeSummary represents aggregated transcode analytics.
type TranscodeSummary struct {
	TotalSessions       int64
	UniqueMedia         int64
	AvgManifestReadyMs  float64
	AvgFirstFrameMs     float64
	AvgFirstSegmentMs   float64
	MinManifestReadyMs  *int64
	MaxManifestReadyMs  *int64
	FailedCount         int64
	HWAccelCount        int64
}

// CorrelatedAnalytics combines frontend and backend analytics for a session.
type CorrelatedAnalytics struct {
	SessionID          string
	MediaID            int64
	FrontendStartupMs  *int64
	TotalPlayTimeMs    *int64
	TotalBufferTimeMs  *int64
	StallCount         *int64
	QualityProfile     *string
	Strategy           *string
	HWAccel            *string
	BackendStartupMs   *int64
	FirstFrameMs       *int64
	FirstSegmentMs     *int64
	TranscodeStatus    *string
	SegmentsCreated    *int64
}

// Repository handles transcode analytics persistence.
type Repository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewRepository creates a new transcode analytics repository.
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

// Create creates a new transcode analytics record when a session starts.
func (r *Repository) Create(ctx context.Context, sessionID string, mediaID int64, quality, strategy, hwAccel string, createdAt int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.CreateTranscodeAnalytics(ctx, sqlc_sqlite.CreateTranscodeAnalyticsParams{
			SessionID:      sessionID,
			MediaID:        mediaID,
			QualityProfile: quality,
			Strategy:       strategy,
			HwAccel:        common.NullString(hwAccel),
			CreatedAt:      createdAt,
		})
	}

	return r.postgresQuerier.CreateTranscodeAnalytics(ctx, sqlc_postgres.CreateTranscodeAnalyticsParams{
		SessionID:      sessionID,
		MediaID:        int32(mediaID),
		QualityProfile: quality,
		Strategy:       strategy,
		HwAccel:        common.NullString(hwAccel),
		CreatedAt:      createdAt,
	})
}

// UpdateFirstFrame records when the first frame was decoded.
func (r *Repository) UpdateFirstFrame(ctx context.Context, sessionID string, firstFrameMs int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateTranscodeFirstFrame(ctx, sqlc_sqlite.UpdateTranscodeFirstFrameParams{
			FirstFrameMs: common.NullInt64(firstFrameMs),
			SessionID:    sessionID,
		})
	}
	return r.postgresQuerier.UpdateTranscodeFirstFrame(ctx, sqlc_postgres.UpdateTranscodeFirstFrameParams{
		FirstFrameMs: common.NullInt32FromInt64(firstFrameMs),
		SessionID:    sessionID,
	})
}

// UpdateFirstSegment records when the first segment was written.
func (r *Repository) UpdateFirstSegment(ctx context.Context, sessionID string, firstSegmentMs int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateTranscodeFirstSegment(ctx, sqlc_sqlite.UpdateTranscodeFirstSegmentParams{
			FirstSegmentMs: common.NullInt64(firstSegmentMs),
			SessionID:      sessionID,
		})
	}
	return r.postgresQuerier.UpdateTranscodeFirstSegment(ctx, sqlc_postgres.UpdateTranscodeFirstSegmentParams{
		FirstSegmentMs: common.NullInt32FromInt64(firstSegmentMs),
		SessionID:      sessionID,
	})
}

// UpdateManifestReady records when the manifest became available.
func (r *Repository) UpdateManifestReady(ctx context.Context, sessionID string, manifestReadyMs int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateTranscodeManifestReady(ctx, sqlc_sqlite.UpdateTranscodeManifestReadyParams{
			ManifestReadyMs: common.NullInt64(manifestReadyMs),
			SessionID:       sessionID,
		})
	}
	return r.postgresQuerier.UpdateTranscodeManifestReady(ctx, sqlc_postgres.UpdateTranscodeManifestReadyParams{
		ManifestReadyMs: common.NullInt32FromInt64(manifestReadyMs),
		SessionID:       sessionID,
	})
}

// UpdateSegmentCount records how many segments have been created.
func (r *Repository) UpdateSegmentCount(ctx context.Context, sessionID string, segmentsCreated int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateTranscodeSegmentCount(ctx, sqlc_sqlite.UpdateTranscodeSegmentCountParams{
			SegmentsCreated: common.NullInt64(segmentsCreated),
			SessionID:       sessionID,
		})
	}
	return r.postgresQuerier.UpdateTranscodeSegmentCount(ctx, sqlc_postgres.UpdateTranscodeSegmentCountParams{
		SegmentsCreated: common.NullInt32FromInt64(segmentsCreated),
		SessionID:       sessionID,
	})
}

// Complete marks a transcode session as completed.
func (r *Repository) Complete(ctx context.Context, sessionID string, totalDurationMs, completedAt int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.CompleteTranscodeAnalytics(ctx, sqlc_sqlite.CompleteTranscodeAnalyticsParams{
			TotalDurationMs: common.NullInt64(totalDurationMs),
			CompletedAt:     common.NullInt64(completedAt),
			SessionID:       sessionID,
		})
	}
	return r.postgresQuerier.CompleteTranscodeAnalytics(ctx, sqlc_postgres.CompleteTranscodeAnalyticsParams{
		TotalDurationMs: common.NullInt32FromInt64(totalDurationMs),
		CompletedAt:     common.NullInt64(completedAt),
		SessionID:       sessionID,
	})
}

// Fail marks a transcode session as failed.
func (r *Repository) Fail(ctx context.Context, sessionID, errorReason string, completedAt int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.FailTranscodeAnalytics(ctx, sqlc_sqlite.FailTranscodeAnalyticsParams{
			ErrorReason: common.NullString(errorReason),
			CompletedAt: common.NullInt64(completedAt),
			SessionID:   sessionID,
		})
	}
	return r.postgresQuerier.FailTranscodeAnalytics(ctx, sqlc_postgres.FailTranscodeAnalyticsParams{
		ErrorReason: common.NullString(errorReason),
		CompletedAt: common.NullInt64(completedAt),
		SessionID:   sessionID,
	})
}

// GetBySessionID retrieves transcode analytics by session ID.
func (r *Repository) GetBySessionID(ctx context.Context, sessionID string) (*TranscodeAnalytics, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetTranscodeAnalyticsBySessionID(ctx, sessionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return &TranscodeAnalytics{
			SessionID:       row.SessionID,
			MediaID:         row.MediaID,
			QualityProfile:  row.QualityProfile,
			Strategy:        row.Strategy,
			HWAccel:         common.ParseNullString(row.HwAccel),
			FirstFrameMs:    common.ParseNullInt64Ptr(row.FirstFrameMs),
			FirstSegmentMs:  common.ParseNullInt64Ptr(row.FirstSegmentMs),
			ManifestReadyMs: common.ParseNullInt64Ptr(row.ManifestReadyMs),
			Status:          row.Status,
			ErrorReason:     common.ParseNullString(row.ErrorReason),
			TotalDurationMs: common.ParseNullInt64Ptr(row.TotalDurationMs),
			SegmentsCreated: common.ParseNullInt64Ptr(row.SegmentsCreated),
			CreatedAt:       row.CreatedAt,
			CompletedAt:     common.ParseNullInt64Ptr(row.CompletedAt),
		}, nil
	}

	row, err := r.postgresQuerier.GetTranscodeAnalyticsBySessionID(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &TranscodeAnalytics{
		SessionID:       row.SessionID,
		MediaID:         int64(row.MediaID),
		QualityProfile:  row.QualityProfile,
		Strategy:        row.Strategy,
		HWAccel:         common.ParseNullString(row.HwAccel),
		FirstFrameMs:    common.ParseNullInt32Ptr(row.FirstFrameMs),
		FirstSegmentMs:  common.ParseNullInt32Ptr(row.FirstSegmentMs),
		ManifestReadyMs: common.ParseNullInt32Ptr(row.ManifestReadyMs),
		Status:          row.Status,
		ErrorReason:     common.ParseNullString(row.ErrorReason),
		TotalDurationMs: common.ParseNullInt32Ptr(row.TotalDurationMs),
		SegmentsCreated: common.ParseNullInt32Ptr(row.SegmentsCreated),
		CreatedAt:       row.CreatedAt,
		CompletedAt:     common.ParseNullInt64Ptr(row.CompletedAt),
	}, nil
}

// GetCorrelatedByMediaID retrieves correlated frontend/backend analytics for a media item.
func (r *Repository) GetCorrelatedByMediaID(ctx context.Context, mediaID int64, limit, offset int) ([]CorrelatedAnalytics, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.GetCorrelatedAnalytics(ctx, sqlc_sqlite.GetCorrelatedAnalyticsParams{
			MediaID: mediaID,
			Limit:   int64(limit),
			Offset:  int64(offset),
		})
		if err != nil {
			return nil, err
		}

		result := make([]CorrelatedAnalytics, len(rows))
		for i, row := range rows {
			result[i] = CorrelatedAnalytics{
				SessionID:          row.SessionID,
				MediaID:            row.MediaID,
				FrontendStartupMs:  common.ParseNullInt64Ptr(row.FrontendStartupMs),
				TotalPlayTimeMs:    common.ParseNullInt64Ptr(row.TotalPlayTimeMs),
				TotalBufferTimeMs:  common.ParseNullInt64Ptr(row.TotalBufferTimeMs),
				StallCount:         common.ParseNullInt64Ptr(row.StallCount),
				QualityProfile:     common.ParseNullStringPtr(row.QualityProfile),
				Strategy:           common.ParseNullStringPtr(row.Strategy),
				HWAccel:            common.ParseNullStringPtr(row.HwAccel),
				BackendStartupMs:   common.ParseNullInt64Ptr(row.BackendStartupMs),
				FirstFrameMs:       common.ParseNullInt64Ptr(row.FirstFrameMs),
				FirstSegmentMs:     common.ParseNullInt64Ptr(row.FirstSegmentMs),
				TranscodeStatus:    common.ParseNullStringPtr(row.TranscodeStatus),
				SegmentsCreated:    common.ParseNullInt64Ptr(row.SegmentsCreated),
			}
		}
		return result, nil
	}

	rows, err := r.postgresQuerier.GetCorrelatedAnalytics(ctx, sqlc_postgres.GetCorrelatedAnalyticsParams{
		MediaID: int32(mediaID),
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]CorrelatedAnalytics, len(rows))
	for i, row := range rows {
		result[i] = CorrelatedAnalytics{
			SessionID:          row.SessionID,
			MediaID:            int64(row.MediaID),
			FrontendStartupMs:  common.ParseNullInt32Ptr(row.FrontendStartupMs),
			TotalPlayTimeMs:    common.ParseNullInt32Ptr(row.TotalPlayTimeMs),
			TotalBufferTimeMs:  common.ParseNullInt32Ptr(row.TotalBufferTimeMs),
			StallCount:         common.ParseNullInt32Ptr(row.StallCount),
			QualityProfile:     common.ParseNullStringPtr(row.QualityProfile),
			Strategy:           common.ParseNullStringPtr(row.Strategy),
			HWAccel:            common.ParseNullStringPtr(row.HwAccel),
			BackendStartupMs:   common.ParseNullInt32Ptr(row.BackendStartupMs),
			FirstFrameMs:       common.ParseNullInt32Ptr(row.FirstFrameMs),
			FirstSegmentMs:     common.ParseNullInt32Ptr(row.FirstSegmentMs),
			TranscodeStatus:    common.ParseNullStringPtr(row.TranscodeStatus),
			SegmentsCreated:    common.ParseNullInt32Ptr(row.SegmentsCreated),
		}
	}
	return result, nil
}

// GetCorrelatedAll retrieves all correlated analytics across all media.
func (r *Repository) GetCorrelatedAll(ctx context.Context, limit, offset int) ([]CorrelatedAnalytics, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.GetCorrelatedAnalyticsAll(ctx, sqlc_sqlite.GetCorrelatedAnalyticsAllParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}

		result := make([]CorrelatedAnalytics, len(rows))
		for i, row := range rows {
			result[i] = CorrelatedAnalytics{
				SessionID:          row.SessionID,
				MediaID:            row.MediaID,
				FrontendStartupMs:  common.ParseNullInt64Ptr(row.FrontendStartupMs),
				TotalPlayTimeMs:    common.ParseNullInt64Ptr(row.TotalPlayTimeMs),
				TotalBufferTimeMs:  common.ParseNullInt64Ptr(row.TotalBufferTimeMs),
				StallCount:         common.ParseNullInt64Ptr(row.StallCount),
				QualityProfile:     common.ParseNullStringPtr(row.QualityProfile),
				Strategy:           common.ParseNullStringPtr(row.Strategy),
				HWAccel:            common.ParseNullStringPtr(row.HwAccel),
				BackendStartupMs:   common.ParseNullInt64Ptr(row.BackendStartupMs),
				FirstFrameMs:       common.ParseNullInt64Ptr(row.FirstFrameMs),
				FirstSegmentMs:     common.ParseNullInt64Ptr(row.FirstSegmentMs),
				TranscodeStatus:    common.ParseNullStringPtr(row.TranscodeStatus),
				SegmentsCreated:    common.ParseNullInt64Ptr(row.SegmentsCreated),
			}
		}
		return result, nil
	}

	rows, err := r.postgresQuerier.GetCorrelatedAnalyticsAll(ctx, sqlc_postgres.GetCorrelatedAnalyticsAllParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]CorrelatedAnalytics, len(rows))
	for i, row := range rows {
		result[i] = CorrelatedAnalytics{
			SessionID:          row.SessionID,
			MediaID:            int64(row.MediaID),
			FrontendStartupMs:  common.ParseNullInt32Ptr(row.FrontendStartupMs),
			TotalPlayTimeMs:    common.ParseNullInt32Ptr(row.TotalPlayTimeMs),
			TotalBufferTimeMs:  common.ParseNullInt32Ptr(row.TotalBufferTimeMs),
			StallCount:         common.ParseNullInt32Ptr(row.StallCount),
			QualityProfile:     common.ParseNullStringPtr(row.QualityProfile),
			Strategy:           common.ParseNullStringPtr(row.Strategy),
			HWAccel:            common.ParseNullStringPtr(row.HwAccel),
			BackendStartupMs:   common.ParseNullInt32Ptr(row.BackendStartupMs),
			FirstFrameMs:       common.ParseNullInt32Ptr(row.FirstFrameMs),
			FirstSegmentMs:     common.ParseNullInt32Ptr(row.FirstSegmentMs),
			TranscodeStatus:    common.ParseNullStringPtr(row.TranscodeStatus),
			SegmentsCreated:    common.ParseNullInt32Ptr(row.SegmentsCreated),
		}
	}
	return result, nil
}
