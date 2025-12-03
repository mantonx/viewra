package transcode

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
)

// ServeAudioPlaylistRequest represents a request for an audio-only HLS playlist.
type ServeAudioPlaylistRequest struct {
	MediaID         int64
	AudioTrackIndex int     // Relative audio track index (0 = first audio, 1 = second audio, etc.)
	OutputDir       string
	StartPosition   float64 // Optional: start position in seconds for seeking
}

// ServeAudioPlaylistResponse represents the result.
type ServeAudioPlaylistResponse struct {
	ManifestPath string
	Reason       string
}

// ServeAudioPlaylistUseCase handles serving audio-only HLS playlists.
type ServeAudioPlaylistUseCase struct {
	mediaRepo      media.Repository
	sessionManager *transcoding.SessionManager
}

// NewServeAudioPlaylistUseCase creates a new ServeAudioPlaylistUseCase.
func NewServeAudioPlaylistUseCase(
	mediaRepo media.Repository,
	sessionManager *transcoding.SessionManager,
) *ServeAudioPlaylistUseCase {
	return &ServeAudioPlaylistUseCase{
		mediaRepo:      mediaRepo,
		sessionManager: sessionManager,
	}
}

// Execute handles the audio playlist serving logic.
func (uc *ServeAudioPlaylistUseCase) Execute(ctx context.Context, req ServeAudioPlaylistRequest) (*ServeAudioPlaylistResponse, error) {
	// Step 1: Get media entity
	mediaEntity, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("media not found: %w", err)
	}

	// Step 2: Get video info to access audio tracks
	videoInfo, err := transcoding.GetVideoInfo(mediaEntity.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze video: %w", err)
	}

	// Step 3: Validate the audio track index exists
	if req.AudioTrackIndex < 0 || req.AudioTrackIndex >= len(videoInfo.AudioTracks) {
		return nil, fmt.Errorf("audio track index %d not found (available: 0-%d)",
			req.AudioTrackIndex, len(videoInfo.AudioTracks)-1)
	}

	// Step 4: Get or create audio-only transcode session
	session, err := uc.sessionManager.GetOrCreateAudioSession(
		req.MediaID,
		req.AudioTrackIndex,
		req.StartPosition,
		mediaEntity.FilePath,
		req.OutputDir,
		videoInfo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio transcode session: %w", err)
	}

	// Wait for FFmpeg to create the manifest file
	if err := session.WaitForManifest(5 * time.Second); err != nil {
		return nil, fmt.Errorf("manifest file not created: %w", err)
	}

	return &ServeAudioPlaylistResponse{
		ManifestPath: session.ManifestPath,
		Reason:       fmt.Sprintf("Audio-only transcoding session for track %d", req.AudioTrackIndex),
	}, nil
}
