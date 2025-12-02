package analytics

import (
	"context"
	"log/slog"
)

// RecordPlaybackUseCase handles recording playback analytics
type RecordPlaybackUseCase struct {
	repo   Repository
	logger *slog.Logger
}

// NewRecordPlaybackUseCase creates a new RecordPlaybackUseCase
func NewRecordPlaybackUseCase(repo Repository, logger *slog.Logger) *RecordPlaybackUseCase {
	return &RecordPlaybackUseCase{
		repo:   repo,
		logger: logger,
	}
}

// RecordPlaybackRequest represents a request to record playback analytics
type RecordPlaybackRequest struct {
	Session *PlaybackSession
	Events  []QualitySwitchEvent
}

// Execute records playback analytics data
func (uc *RecordPlaybackUseCase) Execute(ctx context.Context, req *RecordPlaybackRequest) error {
	// Upsert session if provided
	if req.Session != nil {
		if err := uc.repo.UpsertSession(ctx, req.Session); err != nil {
			uc.logger.Error("failed to upsert session", "error", err, "sessionId", req.Session.SessionID)
			// Continue with events even if session fails
		}
	}

	// Insert events
	if len(req.Events) > 0 {
		if err := uc.repo.CreateEvents(ctx, req.Events); err != nil {
			uc.logger.Error("failed to insert events", "error", err, "count", len(req.Events))
			return err
		}
	}

	return nil
}
