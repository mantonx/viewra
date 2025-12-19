package analytics

import (
	"context"
	"log/slog"
)

// Service provides all analytics-related use cases.
type Service struct {
	repo           Repository
	recordPlayback *RecordPlaybackUseCase
}

// NewService creates a new analytics service.
func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:           repo,
		recordPlayback: NewRecordPlaybackUseCase(repo, logger),
	}
}

// RecordPlayback records playback analytics data.
func (s *Service) RecordPlayback(ctx context.Context, req *RecordPlaybackRequest) error {
	return s.recordPlayback.Execute(ctx, req)
}

// GetSession retrieves a playback session by ID.
func (s *Service) GetSession(ctx context.Context, sessionID string) (*PlaybackSession, error) {
	return s.repo.GetSessionByID(ctx, sessionID)
}

// ListSessions retrieves playback sessions for a media item.
func (s *Service) ListSessions(ctx context.Context, mediaID int64, limit, offset int) ([]PlaybackSession, error) {
	return s.repo.ListSessionsByMediaID(ctx, mediaID, limit, offset)
}

// GetMediaSummary retrieves aggregated analytics for a specific media item.
func (s *Service) GetMediaSummary(ctx context.Context, mediaID int64) (*PlaybackSummary, error) {
	return s.repo.GetSummaryByMediaID(ctx, mediaID)
}

// GetOverallSummary retrieves aggregated analytics across all media.
func (s *Service) GetOverallSummary(ctx context.Context) (*PlaybackSummary, error) {
	return s.repo.GetOverallSummary(ctx)
}
