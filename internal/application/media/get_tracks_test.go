package media

import (
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
)

func TestNewGetTracksUseCase(t *testing.T) {
	uc := NewGetTracksUseCase(nil)
	if uc == nil {
		t.Fatal("NewGetTracksUseCase returned nil")
	}
}

func TestToAudioTrackResponses(t *testing.T) {
	t.Run("nil input returns empty slice", func(t *testing.T) {
		result := toAudioTrackResponses(nil)
		if result == nil {
			t.Fatal("result should not be nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}
	})

	t.Run("converts tracks correctly", func(t *testing.T) {
		now := time.Now()
		tracks := []*media.AudioTrack{
			{
				ID:            1,
				MediaID:       100,
				StreamIndex:   0,
				Codec:         "aac",
				CodecProfile:  "LC",
				Channels:      6,
				ChannelLayout: "5.1",
				SampleRate:    48000,
				BitRate:       384000,
				Language:      "eng",
				Title:         "English 5.1",
				IsDefault:     true,
				IsCommentary:  false,
				IsDescriptive: false,
				CreatedAt:     now,
			},
			{
				ID:            2,
				MediaID:       100,
				StreamIndex:   1,
				Codec:         "ac3",
				Channels:      2,
				ChannelLayout: "stereo",
				Language:      "fra",
				Title:         "French",
				IsDefault:     false,
				IsCommentary:  true,
				CreatedAt:     now,
			},
		}

		result := toAudioTrackResponses(tracks)

		if len(result) != 2 {
			t.Fatalf("expected 2 tracks, got %d", len(result))
		}

		// Check first track
		if result[0].ID != 1 {
			t.Errorf("result[0].ID = %d, want 1", result[0].ID)
		}
		if result[0].Codec != "aac" {
			t.Errorf("result[0].Codec = %q, want %q", result[0].Codec, "aac")
		}
		if result[0].Channels != 6 {
			t.Errorf("result[0].Channels = %d, want 6", result[0].Channels)
		}
		if !result[0].IsDefault {
			t.Error("result[0].IsDefault should be true")
		}

		// Check second track
		if result[1].ID != 2 {
			t.Errorf("result[1].ID = %d, want 2", result[1].ID)
		}
		if !result[1].IsCommentary {
			t.Error("result[1].IsCommentary should be true")
		}
	})
}

func TestToSubtitleTrackResponses(t *testing.T) {
	t.Run("nil input returns empty slice", func(t *testing.T) {
		result := toSubtitleTrackResponses(nil)
		if result == nil {
			t.Fatal("result should not be nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}
	})

	t.Run("converts tracks correctly", func(t *testing.T) {
		now := time.Now()
		streamIdx := 2
		filePath := "/path/to/subtitle.srt"

		tracks := []*media.SubtitleTrack{
			{
				ID:           1,
				MediaID:      100,
				StreamIndex:  &streamIdx,
				SourceType:   media.SubtitleSourceEmbedded,
				Codec:        "subrip",
				Language:     "eng",
				Title:        "English",
				IsDefault:    true,
				IsForced:     false,
				IsSDH:        true,
				IsCommentary: false,
				IsBitmap:     false,
				CreatedAt:    now,
			},
			{
				ID:           2,
				MediaID:      100,
				StreamIndex:  nil,
				SourceType:   media.SubtitleSourceExternal,
				Codec:        "subrip",
				Language:     "spa",
				Title:        "Spanish",
				FilePath:     &filePath,
				IsDefault:    false,
				IsForced:     true,
				IsSDH:        false,
				IsBitmap:     false,
				CreatedAt:    now,
			},
		}

		result := toSubtitleTrackResponses(tracks)

		if len(result) != 2 {
			t.Fatalf("expected 2 tracks, got %d", len(result))
		}

		// Check embedded track
		if result[0].ID != 1 {
			t.Errorf("result[0].ID = %d, want 1", result[0].ID)
		}
		if result[0].SourceType != "embedded" {
			t.Errorf("result[0].SourceType = %q, want %q", result[0].SourceType, "embedded")
		}
		if !result[0].IsSDH {
			t.Error("result[0].IsSDH should be true")
		}
		if result[0].StreamIndex == nil || *result[0].StreamIndex != 2 {
			t.Error("result[0].StreamIndex should be 2")
		}

		// Check external track
		if result[1].ID != 2 {
			t.Errorf("result[1].ID = %d, want 2", result[1].ID)
		}
		if result[1].SourceType != "external" {
			t.Errorf("result[1].SourceType = %q, want %q", result[1].SourceType, "external")
		}
		if !result[1].IsForced {
			t.Error("result[1].IsForced should be true")
		}
		if result[1].FilePath == nil || *result[1].FilePath != filePath {
			t.Errorf("result[1].FilePath = %v, want %q", result[1].FilePath, filePath)
		}
	})
}

func TestAudioTrackResponseCreation(t *testing.T) {
	now := time.Now()
	resp := AudioTrackResponse{
		ID:            1,
		MediaID:       100,
		StreamIndex:   0,
		Codec:         "truehd",
		CodecProfile:  "Atmos",
		Channels:      8,
		ChannelLayout: "7.1",
		SampleRate:    48000,
		BitRate:       0, // Variable
		Language:      "eng",
		Title:         "TrueHD Atmos 7.1",
		IsDefault:     true,
		IsCommentary:  false,
		IsDescriptive: false,
		CreatedAt:     now,
	}

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Codec != "truehd" {
		t.Errorf("Codec = %q, want %q", resp.Codec, "truehd")
	}
	if resp.Channels != 8 {
		t.Errorf("Channels = %d, want 8", resp.Channels)
	}
}

func TestSubtitleTrackResponseCreation(t *testing.T) {
	now := time.Now()
	streamIdx := 5
	resp := SubtitleTrackResponse{
		ID:           1,
		MediaID:      100,
		StreamIndex:  &streamIdx,
		SourceType:   "embedded",
		Codec:        "hdmv_pgs_subtitle",
		Language:     "eng",
		Title:        "English PGS",
		IsDefault:    true,
		IsForced:     false,
		IsSDH:        false,
		IsCommentary: false,
		IsBitmap:     true,
		CreatedAt:    now,
	}

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Codec != "hdmv_pgs_subtitle" {
		t.Errorf("Codec = %q, want %q", resp.Codec, "hdmv_pgs_subtitle")
	}
	if !resp.IsBitmap {
		t.Error("IsBitmap should be true")
	}
}

func TestGetTracksResponseCreation(t *testing.T) {
	resp := GetTracksResponse{
		AudioTracks: []AudioTrackResponse{
			{ID: 1, Codec: "aac", Channels: 6},
			{ID: 2, Codec: "ac3", Channels: 2},
		},
		SubtitleTracks: []SubtitleTrackResponse{
			{ID: 1, SourceType: "embedded", Language: "eng"},
			{ID: 2, SourceType: "external", Language: "spa"},
		},
	}

	if len(resp.AudioTracks) != 2 {
		t.Errorf("AudioTracks count = %d, want 2", len(resp.AudioTracks))
	}
	if len(resp.SubtitleTracks) != 2 {
		t.Errorf("SubtitleTracks count = %d, want 2", len(resp.SubtitleTracks))
	}
}
