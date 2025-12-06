package transcode

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
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
	sessionManager *transcoding.SessionManager
}

// NewServeManifestUseCase creates a new ServeManifestUseCase.
func NewServeManifestUseCase(
	mediaRepo media.Repository,
	sessionManager *transcoding.SessionManager,
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

	// Step 2: Analyze video to get duration and determine strategy
	videoInfo, err := transcoding.GetVideoInfo(mediaEntity.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze video: %w", err)
	}

	// Step 3: Determine streaming strategy
	// Use strategy hint from master playlist if available (ensures consistency)
	// Otherwise determine from client capabilities
	var strategy transcoding.StreamStrategy
	var reason string

	if req.StrategyHint != "" {
		// Use the strategy passed from master playlist
		strategy = transcoding.StreamStrategy(req.StrategyHint)
		reason = fmt.Sprintf("using strategy from master playlist: %s", strategy)
	} else {
		// Determine strategy from client capabilities (direct requests)
		var clientCaps *transcoding.ClientCapabilitiesForStrategy
		if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 {
			clientCaps = &transcoding.ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: req.SupportedVideoCodecs,
				SupportedContainers:  req.SupportedContainers,
			}
		}
		strategy, reason = transcoding.DetermineStreamStrategyWithCapabilities(videoInfo, clientCaps)
	}

	// Step 4: Handle DirectPlay (no transcoding needed)
	if strategy == transcoding.DirectPlay {
		return &ServeManifestResponse{
			Strategy:      StrategyDirectPlay,
			DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
			Reason:        reason,
		}, nil
	}

	// Step 5: Get or create progressive transcode session
	// Get adaptive profile for this quality level
	profile, err := transcoding.GetAdaptiveProfileForQuality(req.Quality)
	if err != nil {
		return nil, fmt.Errorf("invalid quality: %w", err)
	}

	// Create or reuse transcode session
	// Pass client's supported video codecs for intelligent codec selection
	session, err := uc.sessionManager.GetOrCreateSession(
		req.MediaID,
		req.Quality,
		req.StartPosition,
		req.AudioTrackIndex,
		mediaEntity.FilePath,
		profile,
		strategy,
		req.OutputDir,
		videoInfo,
		req.SupportedVideoCodecs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcode session: %w", err)
	}

	// Wait for FFmpeg to create the manifest file (should be very quick - 1-2 seconds max)
	// Wait for FFmpeg to create manifest. 4K HDR content with seeking can take 10-15s
	// to decode the first keyframe and start producing output.
	if err := session.WaitForManifest(20 * time.Second); err != nil {
		return nil, fmt.Errorf("manifest file not created: %w", err)
	}

	// Return manifest path
	return &ServeManifestResponse{
		Strategy:     StrategyServe,
		ManifestPath: session.ManifestPath,
		Reason:       fmt.Sprintf("Progressive transcoding session started from position %.0fs", session.StartPosition),
	}, nil
}
