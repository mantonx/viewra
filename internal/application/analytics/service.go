package analytics

import (
	"context"
	"log/slog"
)

// Service provides all analytics-related use cases.
type Service struct {
	recordPlayback *RecordPlaybackUseCase
}

// NewService creates a new analytics service.
func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		recordPlayback: NewRecordPlaybackUseCase(repo, logger),
	}
}

// RecordPlayback records playback analytics data.
func (s *Service) RecordPlayback(ctx context.Context, req *RecordPlaybackRequest) error {
	return s.recordPlayback.Execute(ctx, req)
}
