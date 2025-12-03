package media

import "time"

// AudioTrack represents an audio stream in a media file.
// Used for multi-language audio support in HLS streaming.
type AudioTrack struct {
	ID            int64
	MediaID       int64
	StreamIndex   int    // FFmpeg stream index
	Codec         string // aac, ac3, eac3, dts, truehd, etc.
	CodecProfile  string // LC, HE-AAC, etc.
	Channels      int    // Number of audio channels (2, 6, 8, etc.)
	ChannelLayout string // stereo, 5.1, 7.1, etc.
	SampleRate    int    // Sample rate in Hz (48000, 44100, etc.)
	BitRate       int    // Bitrate in bits per second
	Language      string // ISO 639-2 language code (eng, jpn, spa, etc.)
	Title         string // Track title from metadata
	IsDefault     bool   // Marked as default track
	IsCommentary  bool   // Director's commentary or similar
	IsDescriptive bool   // Audio description for visually impaired
	CreatedAt     time.Time
}

// SubtitleSourceType indicates where the subtitle comes from.
type SubtitleSourceType string

const (
	SubtitleSourceEmbedded SubtitleSourceType = "embedded"
	SubtitleSourceExternal SubtitleSourceType = "external"
)

// SubtitleTrack represents a subtitle stream (embedded or external).
// Supports both text-based (SRT, VTT, ASS) and bitmap (PGS, VobSub) formats.
type SubtitleTrack struct {
	ID           int64
	MediaID      int64
	StreamIndex  *int               // FFmpeg stream index (nil for external)
	SourceType   SubtitleSourceType // embedded or external
	Codec        string             // subrip, ass, webvtt, hdmv_pgs_subtitle, dvd_subtitle
	Language     string             // ISO 639-2 language code
	Title        string             // Track title from metadata
	FilePath     *string            // Path to external subtitle file (nil for embedded)
	IsDefault    bool               // Marked as default track
	IsForced     bool               // Forced subtitles (translations of foreign dialogue)
	IsSDH        bool               // Subtitles for deaf/hard of hearing
	IsCommentary bool               // Commentary subtitles
	IsBitmap     bool               // Bitmap-based subtitles (PGS, VobSub)
	CreatedAt    time.Time
}

// IsTextBased returns true if the subtitle is text-based and can be converted to WebVTT.
func (s *SubtitleTrack) IsTextBased() bool {
	return !s.IsBitmap
}

// DisplayName returns a human-readable name for the track.
// Uses title if available, otherwise falls back to language.
func (a *AudioTrack) DisplayName() string {
	if a.Title != "" {
		return a.Title
	}
	if a.Language != "" {
		return a.Language
	}
	return "Unknown"
}

// DisplayName returns a human-readable name for the track.
func (s *SubtitleTrack) DisplayName() string {
	if s.Title != "" {
		return s.Title
	}
	if s.Language != "" {
		return s.Language
	}
	return "Unknown"
}

// ChannelDescription returns a human-readable description of the audio channels.
func (a *AudioTrack) ChannelDescription() string {
	switch a.ChannelLayout {
	case "stereo":
		return "Stereo"
	case "5.1", "5.1(side)":
		return "5.1 Surround"
	case "7.1", "7.1(wide)":
		return "7.1 Surround"
	case "mono":
		return "Mono"
	default:
		if a.Channels > 0 {
			switch a.Channels {
			case 1:
				return "Mono"
			case 2:
				return "Stereo"
			case 6:
				return "5.1 Surround"
			case 8:
				return "7.1 Surround"
			default:
				return ""
			}
		}
		return ""
	}
}
