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

	// Client capabilities for smart direct play decisions
	SupportedVideoCodecs []string // e.g., ["h264", "h265", "vp9", "av1"]
	SupportedContainers  []string // e.g., ["mp4", "webm", "matroska"]

	// Subtitle burn-in options for PGS/bitmap subtitles
	BurnInSubtitle      bool // Whether to burn subtitles into the video
	SubtitleStreamIndex int  // Relative index among subtitle streams (0-based)
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

	// Step 3: Determine streaming strategy with client capabilities
	var clientCaps *transcoding.ClientCapabilitiesForStrategy
	if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 {
		clientCaps = &transcoding.ClientCapabilitiesForStrategy{
			SupportedVideoCodecs: req.SupportedVideoCodecs,
			SupportedContainers:  req.SupportedContainers,
		}
	}
	strategy, reason := transcoding.DetermineStreamStrategyWithCapabilities(videoInfo, clientCaps)

	// Step 3b: Force transcode if subtitle burn-in is requested
	// Subtitle burn-in requires video re-encoding - can't be done with remux (copy)
	// PGS/bitmap subtitles ALWAYS use burn-in (client-side extraction is too slow)
	if req.BurnInSubtitle && (strategy == transcoding.Remux || strategy == transcoding.RemuxWithAudioDownmix || strategy == transcoding.DirectPlay) {
		strategy = transcoding.Transcode
		reason = fmt.Sprintf("subtitle burn-in requested, forcing transcode (was: %s)", reason)
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
	// Pass subtitle burn-in options if requested (always enabled for bitmap subtitles)
	var subtitleOpts *transcoding.SubtitleBurnInOptions
	if req.BurnInSubtitle {
		subtitleOpts = &transcoding.SubtitleBurnInOptions{
			Enabled:     true,
			StreamIndex: req.SubtitleStreamIndex,
		}
	}
	session, err := uc.sessionManager.GetOrCreateSession(
		req.MediaID,
		req.Quality,
		req.StartPosition,
		mediaEntity.FilePath,
		profile,
		strategy,
		req.OutputDir,
		videoInfo,
		req.SupportedVideoCodecs,
		subtitleOpts,
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
