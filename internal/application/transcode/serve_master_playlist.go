package transcode

import (
	"context"
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
)

// ServeMasterPlaylistRequest represents a request for the master HLS playlist.
type ServeMasterPlaylistRequest struct {
	MediaID int64

	// Client capabilities for direct play decisions
	SupportedVideoCodecs []string // e.g., ["h264", "h265", "vp9", "av1"]
	SupportedContainers  []string // e.g., ["mp4", "webm", "matroska"]

	// StartPosition for seeking (passed through to variant URLs)
	StartPosition string

	// User preferences for track selection
	PreferredAudioLanguage    string // ISO 639-2 code (eng, fra, jpn, etc.)
	PreferredSubtitleLanguage string // ISO 639-2 code or "off"
}

// ServeMasterPlaylistResponse represents the result.
type ServeMasterPlaylistResponse struct {
	// Strategy indicates what action to take
	Strategy MasterPlaylistStrategy

	// PlaylistContent is the generated M3U8 content (for StrategyServePlaylist)
	PlaylistContent string

	// DirectPlayURL is the URL for direct playback (for StrategyDirectPlay)
	DirectPlayURL string

	// Reason explains why this strategy was chosen
	Reason string
}

// MasterPlaylistStrategy indicates how to handle the master playlist request.
type MasterPlaylistStrategy int

const (
	// StrategyServePlaylist means master playlist was generated
	StrategyServePlaylist MasterPlaylistStrategy = iota

	// StrategyMasterDirectPlay means video is compatible and can be played directly
	StrategyMasterDirectPlay
)

// ABRVariant represents a single quality variant in the ABR ladder.
type ABRVariant struct {
	Quality   string
	Bandwidth int
	Width     int
	Height    int
	Codecs    string
}

// ServeMasterPlaylistUseCase handles generating the HLS master playlist.
type ServeMasterPlaylistUseCase struct {
	mediaRepo media.Repository
}

// NewServeMasterPlaylistUseCase creates a new use case.
func NewServeMasterPlaylistUseCase(mediaRepo media.Repository) *ServeMasterPlaylistUseCase {
	return &ServeMasterPlaylistUseCase{
		mediaRepo: mediaRepo,
	}
}

// Execute generates the master playlist or determines direct play is possible.
func (uc *ServeMasterPlaylistUseCase) Execute(ctx context.Context, req ServeMasterPlaylistRequest) (*ServeMasterPlaylistResponse, error) {
	// Get media to determine source resolution and properties
	mediaItem, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("media not found: %w", err)
	}

	// Get audio tracks for multi-audio support
	audioTracks, err := uc.mediaRepo.GetAudioTracksByMediaID(ctx, req.MediaID)
	if err != nil {
		// Non-fatal: continue without audio track info
		audioTracks = nil
	}

	// Get subtitle tracks for HLS subtitle support
	subtitleTracks, err := uc.mediaRepo.GetSubtitleTracksByMediaID(ctx, req.MediaID)
	if err != nil {
		// Non-fatal: continue without subtitle track info
		subtitleTracks = nil
	}

	// Check if direct play is possible
	videoInfo, err := transcoding.GetVideoInfo(mediaItem.FilePath)
	if err == nil && videoInfo != nil {
		var clientCaps *transcoding.ClientCapabilitiesForStrategy
		if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 {
			clientCaps = &transcoding.ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: req.SupportedVideoCodecs,
				SupportedContainers:  req.SupportedContainers,
			}
		}

		strategy, reason := transcoding.DetermineStreamStrategyWithCapabilities(videoInfo, clientCaps)

		// DirectPlay is allowed - subtitles are handled client-side via overlay
		if strategy == transcoding.DirectPlay {
			return &ServeMasterPlaylistResponse{
				Strategy:      StrategyMasterDirectPlay,
				DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
				Reason:        reason,
			}, nil
		}
	}

	// Build master playlist with audio and subtitle track information
	playlist := uc.buildMasterPlaylist(mediaItem, audioTracks, subtitleTracks, req.StartPosition, req.PreferredAudioLanguage, req.PreferredSubtitleLanguage)

	return &ServeMasterPlaylistResponse{
		Strategy:        StrategyServePlaylist,
		PlaylistContent: playlist,
		Reason:          "Master playlist generated with quality variants and audio tracks",
	}, nil
}

// buildMasterPlaylist creates the HLS master playlist content with multi-audio and subtitle support.
func (uc *ServeMasterPlaylistUseCase) buildMasterPlaylist(mediaItem *media.Media, audioTracks []*media.AudioTrack, subtitleTracks []*media.SubtitleTrack, startPosition string, preferredAudioLang string, preferredSubtitleLang string) string {
	sourceHeight := mediaItem.Height
	sourceWidth := mediaItem.Width
	sourceBitrate := mediaItem.Bitrate

	// Get ABR ladder filtered for this source
	variants := uc.getFilteredABRLadder(sourceWidth, sourceHeight, sourceBitrate)

	// Build playlist header
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:4\n"
	playlist += "#EXT-X-INDEPENDENT-SEGMENTS\n\n"

	// Add audio track renditions if we have multiple audio tracks
	audioGroupID := ""
	if len(audioTracks) > 1 {
		audioGroupID = "audio"
		playlist += uc.buildAudioRenditions(audioTracks, preferredAudioLang, startPosition)
		playlist += "\n"
	}

	// Add subtitle track renditions for text-based subtitles
	// Bitmap subtitles (PGS, VobSub) are handled client-side via WebP overlay
	subtitleGroupID := ""
	textSubtitles := uc.filterTextSubtitles(subtitleTracks)
	if len(textSubtitles) > 0 {
		subtitleGroupID = "subs"
		playlist += uc.buildSubtitleRenditions(textSubtitles, preferredSubtitleLang, startPosition)
		playlist += "\n"
	}

	// Build video stream variants
	for _, variant := range variants {
		streamInf := fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",NAME=\"%s\"",
			variant.Bandwidth,
			variant.Width,
			variant.Height,
			variant.Codecs,
			variant.Quality,
		)

		// Reference audio group if we have multiple audio tracks
		if audioGroupID != "" {
			streamInf += fmt.Sprintf(",AUDIO=\"%s\"", audioGroupID)
		}

		// Reference subtitle group if we have text subtitles
		if subtitleGroupID != "" {
			streamInf += fmt.Sprintf(",SUBTITLES=\"%s\"", subtitleGroupID)
		}

		playlist += streamInf + "\n"

		// Build variant URL with query parameters
		variantURL := fmt.Sprintf("%s/playlist.m3u8", variant.Quality)
		if startPosition != "" {
			variantURL += "?start=" + startPosition
		}
		playlist += variantURL + "\n\n"
	}

	return playlist
}

// filterTextSubtitles returns only text-based subtitles that can be converted to WebVTT.
func (uc *ServeMasterPlaylistUseCase) filterTextSubtitles(subtitleTracks []*media.SubtitleTrack) []*media.SubtitleTrack {
	var textTracks []*media.SubtitleTrack
	for _, track := range subtitleTracks {
		if !track.IsBitmap {
			textTracks = append(textTracks, track)
		}
	}
	return textTracks
}

// buildSubtitleRenditions generates EXT-X-MEDIA tags for subtitle track selection.
func (uc *ServeMasterPlaylistUseCase) buildSubtitleRenditions(subtitleTracks []*media.SubtitleTrack, preferredLang string, startPosition string) string {
	var result strings.Builder

	// Determine which track should be default
	defaultTrackIdx := -1 // -1 means no default (subtitles off by default)
	for i, track := range subtitleTracks {
		// Prefer user's language if specified
		if preferredLang != "" && preferredLang != "off" && track.Language == preferredLang {
			// Prefer forced subtitles if available
			if track.IsForced {
				defaultTrackIdx = i
				break
			}
			// Otherwise use first non-commentary track in preferred language
			if defaultTrackIdx == -1 && !track.IsCommentary {
				defaultTrackIdx = i
			}
		}
	}

	// Build relative index map (among text subtitles only)
	// This is needed because the HLS URI uses the relative text subtitle index
	for i, track := range subtitleTracks {
		isDefault := i == defaultTrackIdx

		// Build track name
		name := uc.buildSubtitleTrackName(track)

		// Convert ISO 639-2 to ISO 639-1 for HLS LANGUAGE attribute
		lang := convertToISO6391(track.Language)

		// Build characteristics for accessibility
		characteristics := ""
		if track.IsSDH {
			characteristics = ",CHARACTERISTICS=\"public.accessibility.transcribes-spoken-dialog,public.accessibility.describes-music-and-sound\""
		}

		// Forced subtitles handling
		forced := ""
		if track.IsForced {
			forced = ",FORCED=YES"
		}

		// The URI points to the subtitle WebVTT file
		// Use the relative subtitle index (position among text subtitles)
		subtitleURI := fmt.Sprintf("subtitle/%d/subtitles.vtt", i)
		if startPosition != "" {
			subtitleURI += "?start=" + startPosition
		}

		result.WriteString(fmt.Sprintf(
			"#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"%s\",LANGUAGE=\"%s\",DEFAULT=%s,AUTOSELECT=%s%s%s,URI=\"%s\"\n",
			name,
			lang,
			boolToYesNo(isDefault),
			boolToYesNo(isDefault),
			forced,
			characteristics,
			subtitleURI,
		))
	}

	return result.String()
}

// buildSubtitleTrackName creates a human-readable name for a subtitle track.
func (uc *ServeMasterPlaylistUseCase) buildSubtitleTrackName(track *media.SubtitleTrack) string {
	// Use title if available
	if track.Title != "" {
		return track.Title
	}

	// Build name from language and characteristics
	name := getLanguageName(track.Language)

	// Add special track types
	if track.IsForced {
		name += " (Forced)"
	}
	if track.IsSDH {
		name += " (SDH)"
	}
	if track.IsCommentary {
		name += " (Commentary)"
	}

	return name
}

// buildAudioRenditions generates EXT-X-MEDIA tags for audio track selection.
func (uc *ServeMasterPlaylistUseCase) buildAudioRenditions(audioTracks []*media.AudioTrack, preferredLang string, startPosition string) string {
	var result strings.Builder

	// Determine which track should be default
	defaultTrackIdx := 0
	for i, track := range audioTracks {
		// Prefer user's language
		if track.Language == preferredLang && !track.IsCommentary {
			defaultTrackIdx = i
			break
		}
		// Fall back to track marked as default
		if track.IsDefault && !track.IsCommentary {
			defaultTrackIdx = i
		}
	}

	for i, track := range audioTracks {
		isDefault := i == defaultTrackIdx

		// Build track name
		name := uc.buildAudioTrackName(track)

		// Convert ISO 639-2 to ISO 639-1 for HLS LANGUAGE attribute
		lang := convertToISO6391(track.Language)

		// Build characteristics for accessibility
		characteristics := ""
		if track.IsDescriptive {
			characteristics = ",CHARACTERISTICS=\"public.accessibility.describes-video\""
		}

		// Use relative audio index (i) for the URI, not the absolute FFmpeg stream index.
		// This matches FFmpeg's -map 0:a:N selector which uses relative audio indices,
		// and aligns with HLS.js's audioTracks array indexing.
		audioURI := fmt.Sprintf("audio/%d/playlist.m3u8", i)
		if startPosition != "" {
			audioURI += "?start=" + startPosition
		}

		result.WriteString(fmt.Sprintf(
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",LANGUAGE=\"%s\",DEFAULT=%s,AUTOSELECT=%s%s,URI=\"%s\"\n",
			name,
			lang,
			boolToYesNo(isDefault),
			boolToYesNo(isDefault),
			characteristics,
			audioURI,
		))
	}

	return result.String()
}

// buildAudioTrackName creates a human-readable name for an audio track.
func (uc *ServeMasterPlaylistUseCase) buildAudioTrackName(track *media.AudioTrack) string {
	// Use title if available
	if track.Title != "" {
		return track.Title
	}

	// Build name from language and characteristics
	name := getLanguageName(track.Language)

	// Add channel layout info
	if track.Channels > 0 {
		switch track.Channels {
		case 1:
			name += " (Mono)"
		case 2:
			name += " (Stereo)"
		case 6:
			name += " (5.1)"
		case 8:
			name += " (7.1)"
		}
	}

	// Add special track types
	if track.IsCommentary {
		name += " - Commentary"
	}
	if track.IsDescriptive {
		name += " - Audio Description"
	}

	return name
}

// boolToYesNo converts a boolean to HLS YES/NO format.
func boolToYesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

// convertToISO6391 converts ISO 639-2 (3-letter) to ISO 639-1 (2-letter) codes.
func convertToISO6391(iso6392 string) string {
	mapping := map[string]string{
		"eng": "en",
		"spa": "es",
		"fra": "fr",
		"deu": "de",
		"ita": "it",
		"por": "pt",
		"jpn": "ja",
		"kor": "ko",
		"zho": "zh",
		"rus": "ru",
		"ara": "ar",
		"hin": "hi",
		"tha": "th",
		"vie": "vi",
		"nld": "nl",
		"pol": "pl",
		"swe": "sv",
		"nor": "no",
		"dan": "da",
		"fin": "fi",
		"tur": "tr",
		"ell": "el",
		"heb": "he",
		"ind": "id",
		"ces": "cs",
		"hun": "hu",
		"ron": "ro",
		"ukr": "uk",
		"und": "und", // Undetermined
	}

	if code, ok := mapping[iso6392]; ok {
		return code
	}
	// Return first 2 chars as fallback
	if len(iso6392) >= 2 {
		return iso6392[:2]
	}
	return "und"
}

// getLanguageName returns the human-readable name for an ISO 639-2 language code.
func getLanguageName(iso6392 string) string {
	names := map[string]string{
		"eng": "English",
		"spa": "Spanish",
		"fra": "French",
		"deu": "German",
		"ita": "Italian",
		"por": "Portuguese",
		"jpn": "Japanese",
		"kor": "Korean",
		"zho": "Chinese",
		"rus": "Russian",
		"ara": "Arabic",
		"hin": "Hindi",
		"tha": "Thai",
		"vie": "Vietnamese",
		"nld": "Dutch",
		"pol": "Polish",
		"swe": "Swedish",
		"nor": "Norwegian",
		"dan": "Danish",
		"fin": "Finnish",
		"tur": "Turkish",
		"ell": "Greek",
		"heb": "Hebrew",
		"ind": "Indonesian",
		"ces": "Czech",
		"hun": "Hungarian",
		"ron": "Romanian",
		"ukr": "Ukrainian",
		"und": "Unknown",
	}

	if name, ok := names[iso6392]; ok {
		return name
	}
	if iso6392 != "" {
		return strings.ToUpper(iso6392)
	}
	return "Unknown"
}

// getFilteredABRLadder returns quality variants appropriate for the source media.
func (uc *ServeMasterPlaylistUseCase) getFilteredABRLadder(sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	// Comprehensive ABR ladder organized by resolution, then by bitrate
	abrLadder := []ABRVariant{
		// 360p - Low quality for poor connections
		{"360p", 800_000, 640, 360, "avc1.4d401e,mp4a.40.2"},

		// 480p - SD quality
		{"480p", 1_500_000, 854, 480, "avc1.4d401e,mp4a.40.2"},
		{"480p-2m", 2_000_000, 854, 480, "avc1.4d401e,mp4a.40.2"},

		// 720p - HD quality (multiple bitrate tiers)
		{"720p-3m", 3_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
		{"720p", 4_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
		{"720p-6m", 6_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},

		// 1080p - Full HD (multiple bitrate tiers)
		{"1080p-6m", 6_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p", 8_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-12m", 12_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-15m", 15_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-20m", 20_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},

		// 4K - Ultra HD (many bitrate tiers for high-quality sources)
		{"4k-20m", 20_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-25m", 25_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-35m", 35_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-50m", 50_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-60m", 60_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-80m", 80_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-100m", 100_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-120m", 120_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	}

	// Add "Original" quality if we know the source bitrate
	if sourceBitrate > 0 && sourceWidth > 0 && sourceHeight > 0 {
		// Round source bitrate up to nearest 5 Mbps for cleaner display
		originalBitrate := ((sourceBitrate / 5_000_000) + 1) * 5_000_000
		if originalBitrate < sourceBitrate {
			originalBitrate = sourceBitrate
		}

		abrLadder = append(abrLadder, ABRVariant{
			Quality:   "original",
			Bandwidth: int(originalBitrate),
			Width:     sourceWidth,
			Height:    sourceHeight,
			Codecs:    "avc1.640033,mp4a.40.2",
		})
	}

	// Filter based on source resolution and bitrate
	return uc.filterVariants(abrLadder, sourceWidth, sourceHeight, sourceBitrate)
}

// filterVariants removes variants that don't make sense for the source media.
func (uc *ServeMasterPlaylistUseCase) filterVariants(variants []ABRVariant, sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	addedVariants := make(map[string]bool)
	var filtered []ABRVariant

	for _, variant := range variants {
		// Skip duplicates
		if addedVariants[variant.Quality] {
			continue
		}

		// Skip variants with bitrate higher than source (except "original")
		if variant.Quality != "original" && sourceBitrate > 0 && int64(variant.Bandwidth) > sourceBitrate {
			continue
		}

		// Skip qualities higher than source if we know resolution
		if sourceHeight > 0 && sourceWidth > 0 {
			// For 4K detection, use width as primary indicator
			// Ultrawide 4K content has full 4K width but reduced height
			is4KSource := sourceWidth >= 3840
			is4KVariant := variant.Width >= 3840

			// Skip 4K variants if source is not 4K
			if is4KVariant && !is4KSource && variant.Quality != "original" {
				continue
			}

			// For non-4K variants, use height-based comparison
			if !is4KVariant && variant.Height > sourceHeight && variant.Quality != "original" {
				continue
			}
		} else {
			// Source resolution unknown - include up to 1080p as safe default
			if variant.Height > 1080 {
				continue
			}
		}

		addedVariants[variant.Quality] = true
		filtered = append(filtered, variant)
	}

	return filtered
}
