// Package transcodeanlytics provides transcode analytics collection and querying.
// It subscribes to transcode events and persists timing metrics for correlation
// with frontend playback analytics.
package transcodeanlytics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	transcoderepo "github.com/mantonx/viewra/internal/infrastructure/persistence/transcode_analytics"
)

// Service handles transcode analytics collection and queries.
type Service struct {
	repo     *transcoderepo.Repository
	eventBus *events.Bus
	logger   *slog.Logger

	mu       sync.Mutex
	cancelFn context.CancelFunc
}

// NewService creates a new transcode analytics service.
func NewService(repo *transcoderepo.Repository, eventBus *events.Bus, logger *slog.Logger) *Service {
	return &Service{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger,
	}
}

// Start begins listening for transcode events.
func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel

	// Subscribe to transcode events
	sub := s.eventBus.Subscribe(
		domainevents.WithBufferSize(100),
		domainevents.WithEventPrefix("transcode."),
	)

	go s.processEvents(ctx, sub)
	s.logger.Info("transcode analytics service started")
}

// Stop stops the event listener.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
	s.logger.Info("transcode analytics service stopped")
}

// processEvents handles incoming transcode events.
func (s *Service) processEvents(ctx context.Context, sub domainevents.Subscription) {
	for {
		select {
		case <-ctx.Done():
			sub.Close()
			return
		case event := <-sub.Events():
			s.handleEvent(ctx, event)
		}
	}
}

// handleEvent processes a single transcode event.
func (s *Service) handleEvent(ctx context.Context, event domainevents.Event) {
	sessionID, ok := event.Data["session_id"].(string)
	if !ok || sessionID == "" {
		return // Can't process without session ID
	}

	switch event.Type {
	case domainevents.EventTranscodeStarted:
		s.handleStarted(ctx, event, sessionID)
	case domainevents.EventTranscodeProgress:
		s.handleProgress(ctx, event, sessionID)
	case domainevents.EventTranscodeCompleted:
		s.handleCompleted(ctx, event, sessionID)
	case domainevents.EventTranscodeFailed:
		s.handleFailed(ctx, event, sessionID)
	}
}

// handleStarted creates a new transcode analytics record.
func (s *Service) handleStarted(ctx context.Context, event domainevents.Event, sessionID string) {
	mediaID, _ := event.Data["media_id"].(int64)
	quality, _ := event.Data["quality"].(string)
	strategy, _ := event.Data["strategy"].(string)
	strategyDisplay, _ := event.Data["strategy_display"].(string)
	strategyReason, _ := event.Data["strategy_reason"].(string)
	hwAccel, _ := event.Data["hw_accel"].(string)

	err := s.repo.Create(ctx, sessionID, mediaID, quality, strategy, strategyDisplay, strategyReason, hwAccel, event.Timestamp.UnixMilli())
	if err != nil {
		s.logger.Error("failed to create transcode analytics", "sessionId", sessionID, "error", err)
	}
}

// handleProgress updates timing milestones.
func (s *Service) handleProgress(ctx context.Context, event domainevents.Event, sessionID string) {
	stage, _ := event.Data["stage"].(string)

	switch stage {
	case "first_frame":
		if timeMs, ok := event.Data["time_to_first_frame_ms"].(int64); ok {
			if err := s.repo.UpdateFirstFrame(ctx, sessionID, timeMs); err != nil {
				s.logger.Error("failed to update first frame time", "sessionId", sessionID, "error", err)
			}
		}
	case "first_segment":
		if timeMs, ok := event.Data["time_to_first_segment_ms"].(int64); ok {
			if err := s.repo.UpdateFirstSegment(ctx, sessionID, timeMs); err != nil {
				s.logger.Error("failed to update first segment time", "sessionId", sessionID, "error", err)
			}
		}
	case "segments":
		if count, ok := event.Data["segments_created"].(int); ok {
			if err := s.repo.UpdateSegmentCount(ctx, sessionID, int64(count)); err != nil {
				s.logger.Error("failed to update segment count", "sessionId", sessionID, "error", err)
			}
		}
	}
}

// handleCompleted marks the session as complete.
func (s *Service) handleCompleted(ctx context.Context, event domainevents.Event, sessionID string) {
	durationMs, _ := event.Data["duration_ms"].(int64)

	err := s.repo.Complete(ctx, sessionID, durationMs, time.Now().UnixMilli())
	if err != nil {
		s.logger.Error("failed to complete transcode analytics", "sessionId", sessionID, "error", err)
	}
}

// handleFailed marks the session as failed.
func (s *Service) handleFailed(ctx context.Context, event domainevents.Event, sessionID string) {
	reason, _ := event.Data["reason"].(string)

	err := s.repo.Fail(ctx, sessionID, reason, time.Now().UnixMilli())
	if err != nil {
		s.logger.Error("failed to mark transcode analytics as failed", "sessionId", sessionID, "error", err)
	}
}

// GetCorrelated retrieves correlated frontend/backend analytics for a media item.
func (s *Service) GetCorrelated(ctx context.Context, mediaID int64, limit, offset int) ([]transcoderepo.CorrelatedAnalytics, error) {
	return s.repo.GetCorrelatedByMediaID(ctx, mediaID, limit, offset)
}

// GetCorrelatedAll retrieves all correlated analytics.
func (s *Service) GetCorrelatedAll(ctx context.Context, limit, offset int) ([]transcoderepo.CorrelatedAnalytics, error) {
	return s.repo.GetCorrelatedAll(ctx, limit, offset)
}

// GetBySessionID retrieves transcode analytics by session ID.
func (s *Service) GetBySessionID(ctx context.Context, sessionID string) (*transcoderepo.TranscodeAnalytics, error) {
	return s.repo.GetBySessionID(ctx, sessionID)
}
