package progress

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// Service provides all progress-related use cases.
// Implements UpdateProgressExecutor, GetProgressExecutor, GetBatchProgressExecutor,
// ListProgressExecutor, MarkWatchedExecutor, and DeleteProgressExecutor.
type Service struct {
	repo      progress.Repository
	prefsRepo progress.PlaybackPreferencesRepository
}

// NewService creates a new progress service
func NewService(repo progress.Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithPreferences creates a new progress service with device-specific preferences support.
func NewServiceWithPreferences(repo progress.Repository, prefsRepo progress.PlaybackPreferencesRepository) *Service {
	return &Service{repo: repo, prefsRepo: prefsRepo}
}

// Execute updates or creates watch progress for a media item (UpdateProgressExecutor)
func (s *Service) Execute(ctx context.Context, req *UpdateProgressRequest) (*WatchProgressResponse, error) {
	// Update watch progress (position, duration, watched status)
	resp, err := UpdateProgress(ctx, s.repo, req)
	if err != nil {
		return nil, err
	}

	// If device profile is provided and we have a preferences repo, save preferences separately
	if req.DeviceProfile != "" && s.prefsRepo != nil {
		hasPreferences := req.SelectedQuality != nil || req.SelectedAudioTrack != nil || req.SelectedSubtitleTrack != nil
		if hasPreferences {
			prefs := &progress.PlaybackPreferences{
				UserID:                req.UserID,
				MediaID:               req.MediaID,
				DeviceProfile:         req.DeviceProfile,
				SelectedQuality:       req.SelectedQuality,
				SelectedAudioTrack:    req.SelectedAudioTrack,
				SelectedSubtitleTrack: req.SelectedSubtitleTrack,
			}
			// Don't fail the whole operation if preferences save fails
			_ = s.prefsRepo.Upsert(ctx, prefs)
		}
	}

	return resp, nil
}

// GetProgress retrieves watch progress for a specific media item (GetProgressExecutor)
func (s *Service) GetProgress(ctx context.Context, mediaID, userID int64) (*WatchProgressResponse, error) {
	return GetProgressByMediaIDAndUserID(ctx, s.repo, mediaID, userID)
}

// GetProgressWithDeviceProfile retrieves watch progress with device-specific preferences.
// If deviceProfile is provided and preferences exist for that profile, they completely replace
// the global preferences in the response. This allows different browsers/devices to have
// different quality settings for the same media.
//
// Important: When a device-specific preferences row exists, its values are used directly,
// even if they are NULL. NULL means "use system default/original quality" for that device.
// Only when no device-specific row exists do we fall back to the base progress preferences.
func (s *Service) GetProgressWithDeviceProfile(ctx context.Context, mediaID, userID int64, deviceProfile string) (*WatchProgressResponse, error) {
	// Get base progress (position, duration, watched status)
	resp, err := GetProgressByMediaIDAndUserID(ctx, s.repo, mediaID, userID)
	if err != nil {
		return nil, err
	}

	// If device profile provided and we have a preferences repo, try to get device-specific preferences
	if deviceProfile != "" && s.prefsRepo != nil {
		prefs, err := s.prefsRepo.Get(ctx, userID, mediaID, deviceProfile)
		if err == nil && prefs != nil {
			// When a device-specific row exists, use its values completely (including NULLs).
			// NULL means "original/default quality" for this device - don't fall back to base.
			// This prevents cross-device preference pollution (e.g., Chrome's transcoded quality
			// being applied to Firefox which can play the original).
			resp.SelectedQuality = prefs.SelectedQuality
			resp.SelectedAudioTrack = prefs.SelectedAudioTrack
			resp.SelectedSubtitleTrack = prefs.SelectedSubtitleTrack
		}
	}

	return resp, nil
}

// GetBatchProgress retrieves watch progress for multiple media items (GetBatchProgressExecutor)
func (s *Service) GetBatchProgress(ctx context.Context, mediaIDs []int64, userID int64) (*BatchProgressResponse, error) {
	return GetBatchProgressByMediaIDs(ctx, s.repo, mediaIDs, userID)
}

// ListProgress retrieves all watch progress records for a user (ListProgressExecutor)
func (s *Service) ListProgress(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListProgressByUserID(ctx, s.repo, userID, limit, offset)
}

// ListWatched retrieves all watched items for a user (ListProgressExecutor)
func (s *Service) ListWatched(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListWatchedByUserID(ctx, s.repo, userID, limit, offset)
}

// ListInProgress retrieves all in-progress items for a user (ListProgressExecutor)
func (s *Service) ListInProgress(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListInProgressByUserID(ctx, s.repo, userID, limit, offset)
}

// MarkWatched marks a media item as fully watched (MarkWatchedExecutor)
func (s *Service) MarkWatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	return MarkWatched(ctx, s.repo, req)
}

// MarkUnwatched resets the watched status for a media item (MarkWatchedExecutor)
func (s *Service) MarkUnwatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	return MarkUnwatched(ctx, s.repo, req)
}

// DeleteProgress deletes watch progress for a media item (DeleteProgressExecutor)
func (s *Service) DeleteProgress(ctx context.Context, mediaID, userID int64) error {
	return DeleteProgress(ctx, s.repo, mediaID, userID)
}
