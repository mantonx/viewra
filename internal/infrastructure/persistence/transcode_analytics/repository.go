package transcodeanlytics

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// TranscodeAnalytics represents a transcode session's analytics data.
type TranscodeAnalytics struct {
	SessionID       string
	MediaID         int64
	QualityProfile  string
	Strategy        string
	StrategyDisplay string // Human-readable strategy name (e.g., "HEVC Remux")
	StrategyReason  string // Detailed reason for the strategy decision
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
	TotalSessions      int64
	UniqueMedia        int64
	AvgManifestReadyMs float64
	AvgFirstFrameMs    float64
	AvgFirstSegmentMs  float64
	MinManifestReadyMs *int64
	MaxManifestReadyMs *int64
	FailedCount        int64
	HWAccelCount       int64
}

// CorrelatedAnalytics combines frontend and backend analytics for a session.
type CorrelatedAnalytics struct {
	SessionID         string
	MediaID           int64
	FrontendStartupMs *int64
	TotalPlayTimeMs   *int64
	TotalBufferTimeMs *int64
	StallCount        *int64
	QualityProfile    *string
	Strategy          *string
	StrategyDisplay   *string
	StrategyReason    *string
	HWAccel           *string
	BackendStartupMs  *int64
	FirstFrameMs      *int64
	FirstSegmentMs    *int64
	TranscodeStatus   *string
	SegmentsCreated   *int64
}

// Repository handles transcode analytics persistence.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new transcode analytics repository.
func NewRepository(baseRepo *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: baseRepo,
	}
}

// Create creates a new transcode analytics record when a session starts.
func (r *Repository) Create(ctx context.Context, sessionID string, mediaID int64, quality, strategy, strategyDisplay, strategyReason, hwAccel string, createdAt int64) error {
	return r.Q().CreateTranscodeAnalytics(ctx, unified.CreateTranscodeAnalyticsParams{
		SessionID:       sessionID,
		MediaID:         mediaID,
		QualityProfile:  quality,
		Strategy:        strategy,
		StrategyDisplay: common.NullString(strategyDisplay),
		StrategyReason:  common.NullString(strategyReason),
		HwAccel:         common.NullString(hwAccel),
		CreatedAt:       createdAt,
	})
}

// UpdateFirstFrame records when the first frame was decoded.
func (r *Repository) UpdateFirstFrame(ctx context.Context, sessionID string, firstFrameMs int64) error {
	return r.Q().UpdateTranscodeFirstFrame(ctx, unified.UpdateTranscodeFirstFrameParams{
		FirstFrameMs: common.NullInt64(firstFrameMs),
		SessionID:    sessionID,
	})
}

// UpdateFirstSegment records when the first segment was written.
func (r *Repository) UpdateFirstSegment(ctx context.Context, sessionID string, firstSegmentMs int64) error {
	return r.Q().UpdateTranscodeFirstSegment(ctx, unified.UpdateTranscodeFirstSegmentParams{
		FirstSegmentMs: common.NullInt64(firstSegmentMs),
		SessionID:      sessionID,
	})
}

// UpdateManifestReady records when the manifest became available.
func (r *Repository) UpdateManifestReady(ctx context.Context, sessionID string, manifestReadyMs int64) error {
	return r.Q().UpdateTranscodeManifestReady(ctx, unified.UpdateTranscodeManifestReadyParams{
		ManifestReadyMs: common.NullInt64(manifestReadyMs),
		SessionID:       sessionID,
	})
}

// UpdateSegmentCount records how many segments have been created.
func (r *Repository) UpdateSegmentCount(ctx context.Context, sessionID string, segmentsCreated int64) error {
	return r.Q().UpdateTranscodeSegmentCount(ctx, unified.UpdateTranscodeSegmentCountParams{
		SegmentsCreated: common.NullInt64(segmentsCreated),
		SessionID:       sessionID,
	})
}

// Complete marks a transcode session as completed.
func (r *Repository) Complete(ctx context.Context, sessionID string, totalDurationMs, completedAt int64) error {
	return r.Q().CompleteTranscodeAnalytics(ctx, unified.CompleteTranscodeAnalyticsParams{
		TotalDurationMs: common.NullInt64(totalDurationMs),
		CompletedAt:     common.NullInt64(completedAt),
		SessionID:       sessionID,
	})
}

// Fail marks a transcode session as failed.
func (r *Repository) Fail(ctx context.Context, sessionID, errorReason string, completedAt int64) error {
	return r.Q().FailTranscodeAnalytics(ctx, unified.FailTranscodeAnalyticsParams{
		ErrorReason: common.NullString(errorReason),
		CompletedAt: common.NullInt64(completedAt),
		SessionID:   sessionID,
	})
}

// GetBySessionID retrieves transcode analytics by session ID.
func (r *Repository) GetBySessionID(ctx context.Context, sessionID string) (*TranscodeAnalytics, error) {
	row, err := r.Q().GetTranscodeAnalyticsBySessionID(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return transcodeAnalyticRowToDomain(row), nil
}

// GetCorrelatedByMediaID retrieves correlated frontend/backend analytics for a media item.
func (r *Repository) GetCorrelatedByMediaID(ctx context.Context, mediaID int64, limit, offset int) ([]CorrelatedAnalytics, error) {
	rows, err := r.Q().GetCorrelatedAnalytics(ctx, unified.GetCorrelatedAnalyticsParams{
		MediaID: mediaID,
		Limit:   int64(limit),
		Offset:  int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, correlatedAnalyticsRowToDomain), nil
}

// GetCorrelatedAll retrieves all correlated analytics across all media.
func (r *Repository) GetCorrelatedAll(ctx context.Context, limit, offset int) ([]CorrelatedAnalytics, error) {
	rows, err := r.Q().GetCorrelatedAnalyticsAll(ctx, unified.GetCorrelatedAnalyticsAllParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, correlatedAnalyticsAllRowToDomain), nil
}

// transcodeAnalyticRowToDomain converts a unified TranscodeAnalytic row to the domain type.
func transcodeAnalyticRowToDomain(row unified.TranscodeAnalytic) *TranscodeAnalytics {
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
	}
}

// correlatedAnalyticsRowToDomain converts a GetCorrelatedAnalyticsRow to the domain type.
func correlatedAnalyticsRowToDomain(row unified.GetCorrelatedAnalyticsRow) CorrelatedAnalytics {
	return CorrelatedAnalytics{
		SessionID:         row.SessionID,
		MediaID:           row.MediaID,
		FrontendStartupMs: common.ParseNullInt64Ptr(row.FrontendStartupMs),
		TotalPlayTimeMs:   common.ParseNullInt64Ptr(row.TotalPlayTimeMs),
		TotalBufferTimeMs: common.ParseNullInt64Ptr(row.TotalBufferTimeMs),
		StallCount:        common.ParseNullInt64Ptr(row.StallCount),
		QualityProfile:    common.ParseNullStringPtr(row.QualityProfile),
		Strategy:          common.ParseNullStringPtr(row.Strategy),
		StrategyDisplay:   common.ParseNullStringPtr(row.StrategyDisplay),
		StrategyReason:    common.ParseNullStringPtr(row.StrategyReason),
		HWAccel:           common.ParseNullStringPtr(row.HwAccel),
		BackendStartupMs:  common.ParseNullInt64Ptr(row.BackendStartupMs),
		FirstFrameMs:      common.ParseNullInt64Ptr(row.FirstFrameMs),
		FirstSegmentMs:    common.ParseNullInt64Ptr(row.FirstSegmentMs),
		TranscodeStatus:   common.ParseNullStringPtr(row.TranscodeStatus),
		SegmentsCreated:   common.ParseNullInt64Ptr(row.SegmentsCreated),
	}
}

// correlatedAnalyticsAllRowToDomain converts a GetCorrelatedAnalyticsAllRow to the domain type.
func correlatedAnalyticsAllRowToDomain(row unified.GetCorrelatedAnalyticsAllRow) CorrelatedAnalytics {
	return CorrelatedAnalytics{
		SessionID:         row.SessionID,
		MediaID:           row.MediaID,
		FrontendStartupMs: common.ParseNullInt64Ptr(row.FrontendStartupMs),
		TotalPlayTimeMs:   common.ParseNullInt64Ptr(row.TotalPlayTimeMs),
		TotalBufferTimeMs: common.ParseNullInt64Ptr(row.TotalBufferTimeMs),
		StallCount:        common.ParseNullInt64Ptr(row.StallCount),
		QualityProfile:    common.ParseNullStringPtr(row.QualityProfile),
		Strategy:          common.ParseNullStringPtr(row.Strategy),
		StrategyDisplay:   common.ParseNullStringPtr(row.StrategyDisplay),
		StrategyReason:    common.ParseNullStringPtr(row.StrategyReason),
		HWAccel:           common.ParseNullStringPtr(row.HwAccel),
		BackendStartupMs:  common.ParseNullInt64Ptr(row.BackendStartupMs),
		FirstFrameMs:      common.ParseNullInt64Ptr(row.FirstFrameMs),
		FirstSegmentMs:    common.ParseNullInt64Ptr(row.FirstSegmentMs),
		TranscodeStatus:   common.ParseNullStringPtr(row.TranscodeStatus),
		SegmentsCreated:   common.ParseNullInt64Ptr(row.SegmentsCreated),
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
