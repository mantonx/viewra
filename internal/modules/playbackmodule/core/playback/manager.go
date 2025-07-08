// Package playback provides the core playback orchestration logic
package playback

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	playbacktypes "github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// Manager orchestrates all playback-related operations
type Manager struct {
	logger                hclog.Logger
	db                    *gorm.DB
	decisionEngine        *DecisionEngine
	deviceDetector        *DeviceDetector
	recommendationTracker *RecommendationTracker
	transcodeDeduplicator *TranscodeDeduplicator
	mediaService          services.MediaService
	transcodingService    services.TranscodingService
}

// NewManager creates a new playback manager
func NewManager(
	logger hclog.Logger,
	db *gorm.DB,
	mediaService services.MediaService,
	transcodingService services.TranscodingService,
) *Manager {
	return &Manager{
		logger:                logger,
		db:                    db,
		decisionEngine:        NewDecisionEngine(logger.Named("decision-engine"), mediaService),
		deviceDetector:        NewDeviceDetector(logger.Named("device-detector")),
		recommendationTracker: NewRecommendationTracker(logger.Named("recommendation"), db),
		transcodeDeduplicator: NewTranscodeDeduplicator(logger.Named("deduplicator"), db),
		mediaService:          mediaService,
		transcodingService:    transcodingService,
	}
}

// DecidePlaybackMethod determines the optimal playback method for a media file
func (m *Manager) DecidePlaybackMethod(ctx context.Context, mediaID string, userAgent string) (*types.PlaybackDecision, error) {
	// Get media file
	mediaFile, err := m.mediaService.GetFile(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media file: %w", err)
	}

	// Create device profile from user agent
	deviceProfile := &types.DeviceProfile{}
	if userAgent != "" {
		// The device detector can enhance the profile based on user agent
		deviceProfile.UserAgent = userAgent
	}

	// Make playback decision
	extendedProfile := &playbacktypes.DeviceProfile{
		DeviceProfile: deviceProfile,
	}
	// The decision engine will use the UserAgent from the embedded DeviceProfile
	decision, err := m.decisionEngine.DecidePlayback(ctx, mediaFile.Path, extendedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to make playback decision: %w", err)
	}

	// Convert internal decision to external types.PlaybackDecision
	result := &types.PlaybackDecision{
		Method:          types.PlaybackMethod(decision.Method),
		DirectPlayURL:   decision.DirectPlayURL,
		TranscodeParams: decision.TranscodeParams,
		Reason:          decision.Reason,
	}

	// Note: Transcode deduplication is handled by the TranscodeDeduplicator.RequestTranscode method
	// when actually starting a transcode, not during decision making

	return result, nil
}

// GetDecisionEngine returns the decision engine (for compatibility)
func (m *Manager) GetDecisionEngine() *DecisionEngine {
	return m.decisionEngine
}

// GetDeviceDetector returns the device detector
func (m *Manager) GetDeviceDetector() *DeviceDetector {
	return m.deviceDetector
}

// GetRecommendationTracker returns the recommendation tracker
func (m *Manager) GetRecommendationTracker() *RecommendationTracker {
	return m.recommendationTracker
}

// GetTranscodeDeduplicator returns the transcode deduplicator
func (m *Manager) GetTranscodeDeduplicator() *TranscodeDeduplicator {
	return m.transcodeDeduplicator
}