package transcode

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
	streamstrategy "github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
)

// ServeManifestRequest represents a request to serve an HLS playlist.
type ServeManifestRequest struct {
	MediaID       int64
	Quality       string
	OutputDir     string
	StartPosition float64 // Optional: start position in seconds for seeking

	// AudioTrackIndex specifies which audio track to mux into the video segments.
	// -1 means use default (first audio track), >= 0 is the FFmpeg stream index.
	AudioTrackIndex int

	// Client capabilities for smart direct play decisions
	SupportedVideoCodecs []string // e.g., ["h264", "h265", "vp9", "av1"]
	SupportedContainers  []string // e.g., ["mp4", "webm", "matroska"]

	// StrategyHint is passed from master playlist to ensure consistent strategy
	// This avoids re-determining strategy when HLS.js requests variant playlists
	StrategyHint string // e.g., "remux_hevc", "transcode"
}

// ServeManifestResponse represents the result of a serve manifest request.
type ServeManifestResponse struct {
	// Strategy indicates what action to take
	Strategy ManifestStrategy

	// ManifestPath is the path to the manifest file (for Serve strategy)
	ManifestPath string

	// DirectPlayURL is the URL for direct playback (for DirectPlay strategy)
	DirectPlayURL string

	// Reason explains why this strategy was chosen
	Reason string

	// SessionID is the transcode session identifier (for analytics correlation)
	// Format: "{mediaID}_{quality}_{unix_timestamp}"
	SessionID string
}

// ManifestStrategy indicates how to handle the manifest request.
type ManifestStrategy int

const (
	// StrategyServe means manifest is generated and segments will be created on-demand
	StrategyServe ManifestStrategy = iota

	// StrategyDirectPlay means video is compatible and can be played directly
	StrategyDirectPlay
)

// ServeManifestUseCase handles serving HLS playlists with progressive transcoding.
type ServeManifestUseCase struct {
	mediaRepo      media.Repository
	sessionManager *session.Manager
}

// NewServeManifestUseCase creates a new ServeManifestUseCase.
func NewServeManifestUseCase(
	mediaRepo media.Repository,
	sessionManager *session.Manager,
) *ServeManifestUseCase {
	return &ServeManifestUseCase{
		mediaRepo:      mediaRepo,
		sessionManager: sessionManager,
	}
}

// Execute handles the playlist serving logic with on-demand segment generation.
func (uc *ServeManifestUseCase) Execute(ctx context.Context, req ServeManifestRequest) (*ServeManifestResponse, error) {
	// Step 1: Get media entity
	mediaEntity, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("media not found: %w", err)
	}

	// Step 2: Get audio tracks for building video info
	audioTracks, err := uc.mediaRepo.GetAudioTracksByMediaID(ctx, req.MediaID)
	if err != nil {
		// Non-fatal: continue without audio track info
		audioTracks = nil
	}

	// Step 3: Build VideoInfo from database instead of calling ffprobe (slow on network files)
	videoInfo := buildVideoInfoFromDatabase(mediaEntity, audioTracks)

	// Step 4: Determine streaming strategy
	// Use strategy hint from master playlist if available (ensures consistency)
	// Otherwise determine from client capabilities
	var streamStrategy streamstrategy.StreamStrategy
	var reason string

	if req.StrategyHint != "" {
		// Use the strategy passed from master playlist
		streamStrategy = streamstrategy.StreamStrategy(req.StrategyHint)
		reason = fmt.Sprintf("using strategy from master playlist: %s", streamStrategy)
	} else {
		// Determine strategy from client capabilities (direct requests)
		var clientCaps *streamstrategy.ClientCapabilities
		if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 {
			clientCaps = &streamstrategy.ClientCapabilities{
				SupportedVideoCodecs: req.SupportedVideoCodecs,
				SupportedContainers:  req.SupportedContainers,
			}
		}
		streamStrategy, reason = streamstrategy.DetermineStrategyWithCapabilities(videoInfo, clientCaps)
	}

	// Step 4b: Override to transcode if quality differs from source resolution
	// Remux strategies copy video as-is, so selecting a different resolution requires transcoding
	isRemuxStrategy := streamStrategy == streamstrategy.Remux ||
		streamStrategy == streamstrategy.RemuxWithAudioDownmix ||
		streamStrategy == streamstrategy.RemuxHEVC

	// Handle quality selection
	var adaptiveProfile *profile.AdaptiveProfile
	isOriginalQuality := req.Quality == "original"

	if isOriginalQuality && isRemuxStrategy {
		// "original" + remux = stream copy at source quality
		// Create a minimal profile - encoding params aren't used for remux
		adaptiveProfile = &profile.AdaptiveProfile{
			ID:              "original",
			DisplayName:     fmt.Sprintf("%dp (Original)", mediaEntity.Height),
			Width:           mediaEntity.Width,
			Height:          mediaEntity.Height,
			VideoBitrate:    int(mediaEntity.Bitrate),
			SegmentDuration: 1, // Required for segment muxer math
		}
	} else if isOriginalQuality && !isRemuxStrategy {
		// "original" + transcode = use highest profile at source resolution
		// This happens when client doesn't support source codec
		adaptiveProfile = profile.GetBestProfileForResolution(mediaEntity.Width, mediaEntity.Height, int(mediaEntity.Bitrate))
		if adaptiveProfile == nil {
			return nil, fmt.Errorf("no suitable profile for resolution %dx%d", mediaEntity.Width, mediaEntity.Height)
		}
	} else {
		var err error
		adaptiveProfile, err = profile.GetAdaptiveProfileForQuality(req.Quality)
		if err != nil {
			return nil, fmt.Errorf("invalid quality: %w", err)
		}

		// Remux can only stream-copy at source quality - any other profile requires transcoding
		if isRemuxStrategy {
			streamStrategy = streamstrategy.Transcode
			reason = fmt.Sprintf("transcoding required: user selected %s (remux only supports original)", req.Quality)
		}
	}
	// For "original" quality with remux strategy, keep the remux strategy as-is
	// The video will be stream-copied at source resolution

	// Step 4: Handle DirectPlay (no transcoding needed)
	if streamStrategy == streamstrategy.DirectPlay {
		return &ServeManifestResponse{
			Strategy:      StrategyDirectPlay,
			DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
			Reason:        reason,
		}, nil
	}

	// Step 5: Get or create progressive transcode session
	// Create or reuse transcode session
	// Pass client's supported video codecs for intelligent codec selection
	transcodeSession, err := uc.sessionManager.GetOrCreateSession(session.GetOrCreateSessionParams{
		MediaID:               req.MediaID,
		Quality:               req.Quality,
		StartPosition:         req.StartPosition,
		AudioTrackIndex:       req.AudioTrackIndex,
		InputPath:             mediaEntity.FilePath,
		Profile:               adaptiveProfile,
		Strategy:              string(streamStrategy),
		OutputDir:             req.OutputDir,
		VideoInfo:             videoInfo,
		ClientSupportedCodecs: req.SupportedVideoCodecs,
		// HWAccel and HWDevice use manager's defaults (from TranscodeConfig)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transcode session: %w", err)
	}

	// Wait for FFmpeg to create the manifest file (should be very quick - 1-2 seconds max)
	// Wait for FFmpeg to create manifest. 4K HDR content with seeking can take 10-15s
	// to decode the first keyframe and start producing output.
	if err := transcodeSession.WaitForManifest(20 * time.Second); err != nil {
		return nil, fmt.Errorf("manifest file not created: %w", err)
	}

	// Return manifest path with session ID for analytics correlation
	return &ServeManifestResponse{
		Strategy:     StrategyServe,
		ManifestPath: transcodeSession.ManifestPath,
		Reason:       fmt.Sprintf("Progressive transcoding session started from position %.0fs", transcodeSession.StartPosition),
		SessionID:    transcodeSession.ID,
	}, nil
}
