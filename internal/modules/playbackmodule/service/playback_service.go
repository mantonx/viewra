// Package service provides the playback service implementation.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/history"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/playback"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/streaming"
	playbacktypes "github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// playbackServiceImpl implements the PlaybackService interface.
// It coordinates playback decisions, transcoding, and session management.
type playbackServiceImpl struct {
	logger             hclog.Logger
	decisionEngine     *playback.DecisionEngine
	progressHandler    *streaming.ProgressiveHandler
	sessionManager     *session.SessionManager
	historyManager     *history.HistoryManager
	mediaService       services.MediaService
	transcodingService services.TranscodingService

	// Event handlers for future module integration
	eventHandlers []PlaybackEventHandler
}

// NewPlaybackService creates a new playback service
// PlaybackEventHandler defines the interface for modules that want to receive playback events
type PlaybackEventHandler interface {
	HandlePlaybackEvent(eventType string, data map[string]interface{}) error
}

func NewPlaybackService(
	logger hclog.Logger,
	decisionEngine *playback.DecisionEngine,
	progressHandler *streaming.ProgressiveHandler,
	sessionManager *session.SessionManager,
	historyManager *history.HistoryManager,
	mediaService services.MediaService,
	transcodingService services.TranscodingService,
) services.PlaybackService {
	return &playbackServiceImpl{
		logger:             logger,
		decisionEngine:     decisionEngine,
		progressHandler:    progressHandler,
		sessionManager:     sessionManager,
		historyManager:     historyManager,
		mediaService:       mediaService,
		transcodingService: transcodingService,
		eventHandlers:      make([]PlaybackEventHandler, 0),
	}
}

// DecidePlayback determines whether to direct play or transcode based on
// media file characteristics and device capabilities.
func (ps *playbackServiceImpl) DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error) {
	ctx := context.Background()

	// Convert to our extended device profile
	extendedProfile := &playbacktypes.DeviceProfile{
		DeviceProfile:        deviceProfile,
		Name:                 "Unknown Device", // Default name
		PreferredContainer:   "mp4",
		PreferredVideoCodec:  "h264",
		PreferredAudioCodec:  "aac",
		SupportedContainers:  deviceProfile.SupportedContainers,
		SupportedVideoCodecs: deviceProfile.SupportedVideoCodecs,
		SupportedAudioCodecs: deviceProfile.SupportedAudioCodecs,
	}
	
	// Fallback to legacy fields if new fields are empty
	if len(extendedProfile.SupportedContainers) == 0 {
		extendedProfile.SupportedContainers = []string{"mp4", "mkv", "webm"}
	}
	if len(extendedProfile.SupportedVideoCodecs) == 0 {
		extendedProfile.SupportedVideoCodecs = deviceProfile.SupportedCodecs
	}
	if len(extendedProfile.SupportedAudioCodecs) == 0 {
		extendedProfile.SupportedAudioCodecs = []string{"aac", "mp3", "opus"}
	}

	ps.logger.Info("Making playback decision",
		"path", mediaPath,
		"device", extendedProfile.Name,
		"supportedVideoCodecs", extendedProfile.SupportedVideoCodecs,
		"supportedAudioCodecs", extendedProfile.SupportedAudioCodecs,
		"supportedContainers", extendedProfile.SupportedContainers)

	// Use decision engine to determine playback method
	decision, err := ps.decisionEngine.DecidePlayback(ctx, mediaPath, extendedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to make playback decision: %w", err)
	}

	// If transcoding is needed, prepare the transcode request
	if decision.Method == "transcode" && decision.TranscodeParams != nil {
		// Get media file info
		mediaFile, err := ps.mediaService.GetFileByPath(ctx, mediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get media file: %w", err)
		}

		// Update transcode params with media ID and input path
		decision.TranscodeParams.MediaID = mediaFile.ID
		decision.TranscodeParams.InputPath = mediaPath
	}

	// Convert back to standard types.PlaybackDecision
	standardDecision := &types.PlaybackDecision{
		Method:          types.PlaybackMethod(decision.Method),
		DirectPlayURL:   decision.DirectPlayURL,
		TranscodeParams: decision.TranscodeParams,
		Reason:          decision.Reason,
	}

	ps.logger.Info("Playback decision complete",
		"method", decision.Method,
		"reason", decision.Reason)

	return standardDecision, nil
}

// RegisterEventHandler allows other modules to register for playback events
// This enables future recommendation engines to receive real-time playback data
func (ps *playbackServiceImpl) RegisterEventHandler(handler PlaybackEventHandler) {
	ps.eventHandlers = append(ps.eventHandlers, handler)
	ps.logger.Info("Registered playback event handler")
}

// publishEvent sends events to all registered handlers
func (ps *playbackServiceImpl) publishEvent(eventType string, data map[string]interface{}) {
	for _, handler := range ps.eventHandlers {
		go func(h PlaybackEventHandler) {
			if err := h.HandlePlaybackEvent(eventType, data); err != nil {
				ps.logger.Error("Event handler failed", "eventType", eventType, "error", err)
			}
		}(handler)
	}
}

// GetUserPlaybackHistory provides access to user behavior data for recommendations
func (ps *playbackServiceImpl) GetUserPlaybackHistory(userID string, mediaType string, limit int) (interface{}, error) {
	if ps.historyManager == nil {
		return nil, fmt.Errorf("history manager not available")
	}

	// Access the MediaHistoryManager through the HistoryManager
	history, err := ps.historyManager.GetMediaHistoryManager().GetPlaybackHistory(userID, mediaType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get playback history: %w", err)
	}

	return history, nil
}

// GetUserPreferences provides access to user preferences for recommendations
func (ps *playbackServiceImpl) GetUserPreferences(userID string) (interface{}, error) {
	if ps.historyManager == nil {
		return nil, fmt.Errorf("history manager not available")
	}

	prefs, err := ps.historyManager.GetMediaHistoryManager().GetUserPreferences(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}

	return prefs, nil
}

// RecordPlaybackInteraction allows recording user interactions for recommendations
func (ps *playbackServiceImpl) RecordPlaybackInteraction(userID, mediaFileID, interactionType string, score float64, context map[string]interface{}) error {
	if ps.historyManager == nil {
		return fmt.Errorf("history manager not available")
	}

	// Record through recommendation tracker
	err := ps.historyManager.GetRecommendationTracker().RecordInteraction(userID, mediaFileID, interactionType, score, context)
	if err != nil {
		return fmt.Errorf("failed to record interaction: %w", err)
	}

	// Publish event for other modules
	eventData := map[string]interface{}{
		"user_id":          userID,
		"media_file_id":    mediaFileID,
		"interaction_type": interactionType,
		"score":            score,
		"context":          context,
		"timestamp":        time.Now(),
	}
	ps.publishEvent("playback.interaction.recorded", eventData)

	return nil
}

// GetMediaInfo analyzes a media file and returns its characteristics.
func (ps *playbackServiceImpl) GetMediaInfo(mediaPath string) (*types.MediaInfo, error) {
	ctx := context.Background()

	ps.logger.Debug("Getting media info", "path", mediaPath)

	// Use media service to get detailed info
	mediaInfo, err := ps.mediaService.GetMediaInfo(ctx, mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get media info: %w", err)
	}

	return mediaInfo, nil
}

// ValidatePlayback checks if a media file can be played on the given device.
func (ps *playbackServiceImpl) ValidatePlayback(mediaPath string, deviceProfile *types.DeviceProfile) error {
	ctx := context.Background()

	// Convert to our extended device profile
	extendedProfile := &playbacktypes.DeviceProfile{
		DeviceProfile:        deviceProfile,
		Name:                 "Unknown Device",
		SupportedContainers:  deviceProfile.SupportedContainers,
		SupportedVideoCodecs: deviceProfile.SupportedVideoCodecs,
		SupportedAudioCodecs: deviceProfile.SupportedAudioCodecs,
	}
	
	// Fallback to legacy fields if new fields are empty
	if len(extendedProfile.SupportedContainers) == 0 {
		extendedProfile.SupportedContainers = []string{"mp4", "mkv", "webm"}
	}
	if len(extendedProfile.SupportedVideoCodecs) == 0 {
		extendedProfile.SupportedVideoCodecs = deviceProfile.SupportedCodecs
	}
	if len(extendedProfile.SupportedAudioCodecs) == 0 {
		extendedProfile.SupportedAudioCodecs = []string{"aac", "mp3", "opus"}
	}

	ps.logger.Debug("Validating playback",
		"path", mediaPath,
		"device", extendedProfile.Name)

	// Use decision engine to validate
	err := ps.decisionEngine.ValidatePlayback(ctx, mediaPath, extendedProfile)
	if err != nil {
		return fmt.Errorf("playback validation failed: %w", err)
	}

	return nil
}

// GetSupportedFormats returns formats supported for direct playback.
func (ps *playbackServiceImpl) GetSupportedFormats(deviceProfile *types.DeviceProfile) []string {
	// Convert to our extended device profile
	extendedProfile := &playbacktypes.DeviceProfile{
		DeviceProfile:       deviceProfile,
		Name:                "Unknown Device",
		SupportedContainers: deviceProfile.SupportedContainers,
	}
	
	// Fallback to legacy fields if new fields are empty
	if len(extendedProfile.SupportedContainers) == 0 {
		extendedProfile.SupportedContainers = []string{"mp4", "mkv", "webm"}
	}

	ps.logger.Debug("Getting supported formats", "device", extendedProfile.Name)

	return ps.decisionEngine.GetSupportedFormats(extendedProfile)
}

// GetRecommendedTranscodeParams returns optimal transcoding parameters if needed.
func (ps *playbackServiceImpl) GetRecommendedTranscodeParams(mediaPath string, deviceProfile *types.DeviceProfile) (*plugins.TranscodeRequest, error) {
	ctx := context.Background()

	// Convert to our extended device profile
	extendedProfile := &playbacktypes.DeviceProfile{
		DeviceProfile:        deviceProfile,
		Name:                 "Unknown Device",
		PreferredContainer:   "mp4",
		PreferredVideoCodec:  "h264",
		PreferredAudioCodec:  "aac",
		SupportedContainers:  deviceProfile.SupportedContainers,
		SupportedVideoCodecs: deviceProfile.SupportedVideoCodecs,
		SupportedAudioCodecs: deviceProfile.SupportedAudioCodecs,
	}
	
	// Fallback to legacy fields if new fields are empty
	if len(extendedProfile.SupportedContainers) == 0 {
		extendedProfile.SupportedContainers = []string{"mp4", "mkv", "webm"}
	}
	if len(extendedProfile.SupportedVideoCodecs) == 0 {
		extendedProfile.SupportedVideoCodecs = deviceProfile.SupportedCodecs
	}
	if len(extendedProfile.SupportedAudioCodecs) == 0 {
		extendedProfile.SupportedAudioCodecs = []string{"aac", "mp3", "opus"}
	}

	ps.logger.Debug("Getting recommended transcode params",
		"path", mediaPath,
		"device", extendedProfile.Name)

	// First make a playback decision
	decision, err := ps.decisionEngine.DecidePlayback(ctx, mediaPath, extendedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to make playback decision: %w", err)
	}

	// If direct play is possible, no transcoding needed
	if decision.Method == "direct" {
		return nil, fmt.Errorf("direct play is possible, no transcoding needed")
	}

	// Return the transcode parameters
	if decision.TranscodeParams == nil {
		return nil, fmt.Errorf("no transcode parameters available")
	}

	return decision.TranscodeParams, nil
}

// StartPlaybackSession starts a new playback session for tracking.
func (ps *playbackServiceImpl) StartPlaybackSession(mediaFileID, userID, deviceID, method string) (*session.PlaybackSession, error) {
	ps.logger.Info("Starting playback session",
		"mediaFileID", mediaFileID,
		"method", method)

	session, err := ps.sessionManager.CreateSession(mediaFileID, userID, deviceID, method)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// UpdatePlaybackSession updates an existing playback session.
func (ps *playbackServiceImpl) UpdatePlaybackSession(sessionID string, updates map[string]interface{}) error {
	ps.logger.Debug("Updating playback session", "sessionID", sessionID)

	err := ps.sessionManager.UpdateSession(sessionID, updates)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// EndPlaybackSession ends a playback session.
func (ps *playbackServiceImpl) EndPlaybackSession(sessionID string) error {
	ps.logger.Info("Ending playback session", "sessionID", sessionID)

	err := ps.sessionManager.EndSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	return nil
}

// GetPlaybackSession retrieves a playback session by ID.
func (ps *playbackServiceImpl) GetPlaybackSession(sessionID string) (*session.PlaybackSession, error) {
	session, err := ps.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	return session, nil
}

// GetActiveSessions returns all active playback sessions.
func (ps *playbackServiceImpl) GetActiveSessions() []*session.PlaybackSession {
	return ps.sessionManager.GetActiveSessions()
}

// PrepareStreamURL prepares a streaming URL based on the playback decision.
func (ps *playbackServiceImpl) PrepareStreamURL(decision *types.PlaybackDecision, baseURL string) (string, error) {
	// Get method from decision
	method := string(decision.Method)

	switch method {
	case "direct":
		// For direct play, return the direct play URL if provided
		if decision.DirectPlayURL != "" {
			return decision.DirectPlayURL, nil
		}
		// Otherwise construct a basic streaming URL
		return fmt.Sprintf("%s/api/v1/playback/stream/direct", baseURL), nil

	case "remux":
		// For remux, we need to start a remux operation
		// This is simplified - in reality you'd start a remux job
		return fmt.Sprintf("%s/api/v1/playback/stream/remux?container=mp4",
			baseURL), nil

	case "transcode":
		// For transcode, start a transcode session
		if decision.TranscodeParams != nil {
			ctx := context.Background()
			// Cast the interface{} to *plugins.TranscodeRequest
			transcodeReq, ok := decision.TranscodeParams.(*plugins.TranscodeRequest)
			if !ok {
				return "", fmt.Errorf("invalid transcode parameters type")
			}

			session, err := ps.transcodingService.StartTranscode(ctx, transcodeReq)
			if err != nil {
				return "", fmt.Errorf("failed to start transcode: %w", err)
			}

			// Return URL for transcoded content
			return fmt.Sprintf("%s/api/v1/content/%s/output.%s",
				baseURL, session.ContentHash, transcodeReq.Container), nil
		}
		return "", fmt.Errorf("no transcode parameters available")

	default:
		return "", fmt.Errorf("unknown playback method: %s", method)
	}
}
