package media

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
)

// GetTracksUseCase handles the business logic for retrieving audio and subtitle tracks for a media item
type GetTracksUseCase struct {
	repo media.Repository
}

// NewGetTracksUseCase creates a new instance of GetTracksUseCase
func NewGetTracksUseCase(repo media.Repository) *GetTracksUseCase {
	return &GetTracksUseCase{
		repo: repo,
	}
}

// AudioTrackResponse represents an audio track in API responses
type AudioTrackResponse struct {
	ID            int64     `json:"id"`
	MediaID       int64     `json:"media_id"`
	StreamIndex   int       `json:"stream_index"`
	Codec         string    `json:"codec"`
	CodecProfile  string    `json:"codec_profile,omitempty"`
	Channels      int       `json:"channels"`
	ChannelLayout string    `json:"channel_layout,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	BitRate       int       `json:"bit_rate,omitempty"`
	Language      string    `json:"language,omitempty"`
	Title         string    `json:"title,omitempty"`
	IsDefault     bool      `json:"is_default"`
	IsCommentary  bool      `json:"is_commentary"`
	IsDescriptive bool      `json:"is_descriptive"`
	CreatedAt     time.Time `json:"created_at"`
}

// SubtitleTrackResponse represents a subtitle track in API responses
type SubtitleTrackResponse struct {
	ID           int64     `json:"id"`
	MediaID      int64     `json:"media_id"`
	StreamIndex  *int      `json:"stream_index,omitempty"`
	SourceType   string    `json:"source_type"` // "embedded" or "external"
	Codec        string    `json:"codec,omitempty"`
	Language     string    `json:"language,omitempty"`
	Title        string    `json:"title,omitempty"`
	FilePath     *string   `json:"file_path,omitempty"`
	IsDefault    bool      `json:"is_default"`
	IsForced     bool      `json:"is_forced"`
	IsSDH        bool      `json:"is_sdh"`
	IsCommentary bool      `json:"is_commentary"`
	IsBitmap     bool      `json:"is_bitmap"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetTracksResponse contains both audio and subtitle tracks for a media item
type GetTracksResponse struct {
	AudioTracks    []AudioTrackResponse    `json:"audio_tracks"`
	SubtitleTracks []SubtitleTrackResponse `json:"subtitle_tracks"`
}

// GetSubtitleTrackByID finds a subtitle track by its database ID.
// Returns nil if no track found with that ID.
func (uc *GetTracksUseCase) GetSubtitleTrackByID(ctx context.Context, mediaID int64, trackID int64) (*media.SubtitleTrack, error) {
	tracks, err := uc.repo.GetSubtitleTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtitle tracks: %w", err)
	}

	for _, t := range tracks {
		if t.ID == trackID {
			return t, nil
		}
	}
	return nil, nil
}

// GetSubtitleTrackByRelativeIndex finds a subtitle track by its relative index among tracks of the same type.
// If bitmap is true, searches among bitmap (PGS) tracks; otherwise searches among text tracks.
// Tracks are sorted by stream index. Returns nil if no track found at the given index.
func (uc *GetTracksUseCase) GetSubtitleTrackByRelativeIndex(ctx context.Context, mediaID int64, relativeIndex int, bitmap bool) (*media.SubtitleTrack, error) {
	tracks, err := uc.repo.GetSubtitleTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtitle tracks: %w", err)
	}

	// Filter by type and count to relative index
	count := 0
	for _, t := range tracks {
		if t.IsBitmap == bitmap && t.StreamIndex != nil {
			if count == relativeIndex {
				return t, nil
			}
			count++
		}
	}
	return nil, nil
}

// Execute retrieves all audio and subtitle tracks for a media item
func (uc *GetTracksUseCase) Execute(ctx context.Context, mediaID int64) (*GetTracksResponse, error) {
	// Verify media exists
	_, err := uc.repo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	// Get audio tracks
	audioTracks, err := uc.repo.GetAudioTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio tracks: %w", err)
	}

	// Get subtitle tracks
	subtitleTracks, err := uc.repo.GetSubtitleTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtitle tracks: %w", err)
	}

	return &GetTracksResponse{
		AudioTracks:    toAudioTrackResponses(audioTracks),
		SubtitleTracks: toSubtitleTrackResponses(subtitleTracks),
	}, nil
}

// toAudioTrackResponses converts domain audio tracks to response DTOs
func toAudioTrackResponses(tracks []*media.AudioTrack) []AudioTrackResponse {
	if tracks == nil {
		return []AudioTrackResponse{}
	}

	responses := make([]AudioTrackResponse, len(tracks))
	for i, t := range tracks {
		responses[i] = AudioTrackResponse{
			ID:            t.ID,
			MediaID:       t.MediaID,
			StreamIndex:   t.StreamIndex,
			Codec:         t.Codec,
			CodecProfile:  t.CodecProfile,
			Channels:      t.Channels,
			ChannelLayout: t.ChannelLayout,
			SampleRate:    t.SampleRate,
			BitRate:       t.BitRate,
			Language:      t.Language,
			Title:         t.Title,
			IsDefault:     t.IsDefault,
			IsCommentary:  t.IsCommentary,
			IsDescriptive: t.IsDescriptive,
			CreatedAt:     t.CreatedAt,
		}
	}
	return responses
}

// toSubtitleTrackResponses converts domain subtitle tracks to response DTOs
func toSubtitleTrackResponses(tracks []*media.SubtitleTrack) []SubtitleTrackResponse {
	if tracks == nil {
		return []SubtitleTrackResponse{}
	}

	responses := make([]SubtitleTrackResponse, len(tracks))
	for i, t := range tracks {
		responses[i] = SubtitleTrackResponse{
			ID:           t.ID,
			MediaID:      t.MediaID,
			StreamIndex:  t.StreamIndex,
			SourceType:   string(t.SourceType),
			Codec:        t.Codec,
			Language:     t.Language,
			Title:        t.Title,
			FilePath:     t.FilePath,
			IsDefault:    t.IsDefault,
			IsForced:     t.IsForced,
			IsSDH:        t.IsSDH,
			IsCommentary: t.IsCommentary,
			IsBitmap:     t.IsBitmap,
			CreatedAt:    t.CreatedAt,
		}
	}
	return responses
}
