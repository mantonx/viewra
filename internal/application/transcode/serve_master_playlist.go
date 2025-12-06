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

	// AudioTrackIndex specifies which audio track to mux into segments
	// -1 means default (first audio track), >= 0 is the FFmpeg stream index
	AudioTrackIndex int

	// User preferences for track selection
	PreferredAudioLanguage    string // ISO 639-2 code (eng, fra, jpn, etc.)
	PreferredSubtitleLanguage string // ISO 639-2 code or "off"
}

// buildVariantParams contains parameters passed to variant playlist URLs.
type buildVariantParams struct {
	startPosition        string
	strategy             transcoding.StreamStrategy
	supportedVideoCodecs []string
	audioTrackIndex      int // -1 means default, >= 0 is FFmpeg stream index
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

// ABRVariant is imported from transcoding package - single source of truth
// Re-export for use in this package
type ABRVariant = transcoding.ABRVariant

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

		// Build master playlist with strategy encoded in variant URLs
		// This ensures HLS.js requests use the correct strategy even without codec headers
		variantParams := buildVariantParams{
			startPosition:        req.StartPosition,
			strategy:             strategy,
			supportedVideoCodecs: req.SupportedVideoCodecs,
			audioTrackIndex:      req.AudioTrackIndex,
		}
		playlist := uc.buildMasterPlaylist(mediaItem, audioTracks, subtitleTracks, variantParams, req.PreferredAudioLanguage, req.PreferredSubtitleLanguage)

		return &ServeMasterPlaylistResponse{
			Strategy:        StrategyServePlaylist,
			PlaylistContent: playlist,
			Reason:          fmt.Sprintf("Master playlist generated with strategy: %s", strategy),
		}, nil
	}

	// Fallback: build playlist without strategy (will default to transcode)
	variantParams := buildVariantParams{
		startPosition:        req.StartPosition,
		strategy:             transcoding.Transcode,
		supportedVideoCodecs: req.SupportedVideoCodecs,
		audioTrackIndex:      req.AudioTrackIndex,
	}
	playlist := uc.buildMasterPlaylist(mediaItem, audioTracks, subtitleTracks, variantParams, req.PreferredAudioLanguage, req.PreferredSubtitleLanguage)

	return &ServeMasterPlaylistResponse{
		Strategy:        StrategyServePlaylist,
		PlaylistContent: playlist,
		Reason:          "Master playlist generated with quality variants and audio tracks",
	}, nil
}

// buildMasterPlaylist creates the HLS master playlist content with multi-audio and subtitle support.
func (uc *ServeMasterPlaylistUseCase) buildMasterPlaylist(mediaItem *media.Media, audioTracks []*media.AudioTrack, subtitleTracks []*media.SubtitleTrack, params buildVariantParams, preferredAudioLang string, preferredSubtitleLang string) string {
	sourceHeight := mediaItem.Height
	sourceWidth := mediaItem.Width
	sourceBitrate := mediaItem.Bitrate

	// Get ABR ladder filtered for this source
	variants := uc.getFilteredABRLadder(sourceWidth, sourceHeight, sourceBitrate)

	// Build playlist header
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:4\n"
	playlist += "#EXT-X-INDEPENDENT-SEGMENTS\n\n"

	// NOTE: We intentionally do NOT add separate audio renditions (EXT-X-MEDIA:TYPE=AUDIO).
	// The video segments already contain muxed audio with perfect A/V sync.
	// Separate audio renditions cause sync issues because:
	// 1. They require separate FFmpeg sessions with independent seeking
	// 2. Audio keyframe alignment differs from video keyframe alignment
	// 3. HLS.js ignores muxed audio when separate renditions are declared
	//
	// For multi-audio track selection, the frontend should request a new transcode
	// session with the desired audio track muxed into the video segments.
	audioGroupID := ""
	_ = audioTracks // Available for future use (track switching via session restart)

	// Add subtitle track renditions for text-based subtitles
	// Bitmap subtitles (PGS, VobSub) are handled client-side via WebP overlay
	subtitleGroupID := ""
	textSubtitles := uc.filterTextSubtitles(subtitleTracks)
	if len(textSubtitles) > 0 {
		subtitleGroupID = "subs"
		playlist += uc.buildSubtitleRenditions(textSubtitles, preferredSubtitleLang, params.startPosition)
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
			variant.ID,
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

		// Build variant URL with query parameters including strategy, codecs, and audio track
		// This ensures HLS.js requests use the correct strategy without needing headers
		variantURL := fmt.Sprintf("%s/playlist.m3u8", variant.ID)
		queryParams := []string{}
		if params.startPosition != "" {
			queryParams = append(queryParams, "start="+params.startPosition)
		}
		if params.strategy != "" {
			queryParams = append(queryParams, "strategy="+string(params.strategy))
		}
		if len(params.supportedVideoCodecs) > 0 {
			queryParams = append(queryParams, "codecs="+strings.Join(params.supportedVideoCodecs, ","))
		}
		// Include audio track selection for multi-audio support
		// Only include if non-default (>= 0) since -1 means use default first track
		if params.audioTrackIndex >= 0 {
			queryParams = append(queryParams, fmt.Sprintf("audioTrack=%d", params.audioTrackIndex))
		}
		if len(queryParams) > 0 {
			variantURL += "?" + strings.Join(queryParams, "&")
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
// Sources are mapped to the nearest appropriate tier based on their resolution and bitrate.
// No "original" quality - sources fall into the appropriate tier for consistent caching.
func (uc *ServeMasterPlaylistUseCase) getFilteredABRLadder(sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	// Use the shared ABR ladder from transcoding package (single source of truth)
	return uc.filterVariants(transcoding.ABRLadder, sourceWidth, sourceHeight, sourceBitrate)
}

// filterVariants selects the single best quality variant for the source media.
// Only one video quality is included to minimize FFmpeg processes and ensure fast startup.
// The player will use this single quality; ABR switching is not supported in this mode.
func (uc *ServeMasterPlaylistUseCase) filterVariants(variants []ABRVariant, sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	// Find the best matching variant for this source
	var bestVariant *ABRVariant

	for i := range variants {
		variant := &variants[i]

		// Skip variants with bitrate significantly higher than source (allow 10% tolerance)
		if sourceBitrate > 0 && int64(variant.Bandwidth) > (sourceBitrate*110/100) {
			continue
		}

		// Skip qualities higher than source if we know resolution
		if sourceHeight > 0 && sourceWidth > 0 {
			is4KSource := sourceWidth >= 3840
			is4KVariant := variant.Width >= 3840

			// Skip 4K variants if source is not 4K
			if is4KVariant && !is4KSource {
				continue
			}

			// For non-4K variants, use height-based comparison
			if !is4KVariant && variant.Height > sourceHeight {
				continue
			}
		} else {
			// Source resolution unknown - include up to 1080p as safe default
			if variant.Height > 1080 {
				continue
			}
		}

		// Select the highest quality variant that passes all filters
		// Variants are ordered from lowest to highest in ABRLadder
		bestVariant = variant
	}

	// Return single best variant (or empty if none matched)
	if bestVariant != nil {
		return []ABRVariant{*bestVariant}
	}

	// Fallback: return the lowest quality if nothing matched
	if len(variants) > 0 {
		return []ABRVariant{variants[0]}
	}

	return nil
}
