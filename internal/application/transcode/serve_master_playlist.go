package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
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

	// Client capabilities for quality recommendation
	ScreenWidth  int   // Screen width in pixels (accounting for device pixel ratio)
	ScreenHeight int   // Screen height in pixels (accounting for device pixel ratio)
	Bandwidth    int64 // Estimated bandwidth in bits per second

	// QualityOverride allows user to manually select a specific quality
	// If set, bypasses recommendation logic and uses this quality directly
	QualityOverride string // e.g., "4k-25m", "1080p-10m", "720p-4m"

	// ClientSupportsHDR indicates the client has an HDR-capable display
	// If true and client supports HEVC, HDR content can be remuxed without tone mapping
	ClientSupportsHDR bool
}

// buildVariantParams contains parameters passed to variant playlist URLs.
type buildVariantParams struct {
	startPosition        string
	strategy             strategy.StreamStrategy
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

	// AvailableQualities lists all quality options valid for this media (filtered by source resolution)
	// Frontend uses this for the quality picker UI
	AvailableQualities []QualityOption
}

// QualityOption represents a selectable quality for the frontend picker.
type QualityOption struct {
	ID          string `json:"id"`          // Quality ID (e.g., "1080p-10m", "4k-25m")
	DisplayName string `json:"displayName"` // Human-readable name (e.g., "1080p (10 Mbps)")
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Bandwidth   int    `json:"bandwidth"` // Video bitrate in bps
	IsSelected  bool   `json:"isSelected"` // True if this is the currently selected quality
}

// MasterPlaylistStrategy indicates how to handle the master playlist request.
type MasterPlaylistStrategy int

const (
	// StrategyServePlaylist means master playlist was generated
	StrategyServePlaylist MasterPlaylistStrategy = iota

	// StrategyMasterDirectPlay means video is compatible and can be played directly
	StrategyMasterDirectPlay
)

// ABRVariant is imported from profile package - single source of truth
// Re-export for use in this package
type ABRVariant = profile.ABRVariant

// ServeMasterPlaylistUseCase handles generating the HLS master playlist.
type ServeMasterPlaylistUseCase struct {
	mediaRepo   media.Repository
	recommender *profile.QualityRecommender
}

// NewServeMasterPlaylistUseCase creates a new use case.
func NewServeMasterPlaylistUseCase(mediaRepo media.Repository, recommender *profile.QualityRecommender) *ServeMasterPlaylistUseCase {
	return &ServeMasterPlaylistUseCase{
		mediaRepo:   mediaRepo,
		recommender: recommender,
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

	// Build VideoInfo from database instead of calling ffprobe (which is slow on network files)
	videoInfo := buildVideoInfoFromDatabase(mediaItem, audioTracks)

	// Determine streaming strategy based on client capabilities
	var clientCaps *strategy.ClientCapabilities
	if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 || req.ClientSupportsHDR {
		clientCaps = &strategy.ClientCapabilities{
			SupportedVideoCodecs: req.SupportedVideoCodecs,
			SupportedContainers:  req.SupportedContainers,
			SupportsHDRDisplay:   req.ClientSupportsHDR,
		}
	}

	streamStrategy, reason := strategy.DetermineStrategyWithCapabilities(videoInfo, clientCaps)

	// Debug logging for strategy decision
	slog.Info("strategy decision",
		"mediaID", req.MediaID,
		"codec", videoInfo.Codec,
		"isHDR", videoInfo.IsHDR,
		"clientSupportsHDR", req.ClientSupportsHDR,
		"supportedCodecs", req.SupportedVideoCodecs,
		"strategy", streamStrategy,
		"reason", reason,
	)

	// DirectPlay is allowed - subtitles are handled client-side via overlay
	if streamStrategy == strategy.DirectPlay {
		return &ServeMasterPlaylistResponse{
			Strategy:      StrategyMasterDirectPlay,
			DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
			Reason:        reason,
		}, nil
	}

	// Determine the quality to use
	selectedQuality := uc.determineQuality(req, mediaItem)

	// Build available qualities list (filtered by source resolution)
	availableQualities := uc.buildAvailableQualities(mediaItem, selectedQuality)

	// Build single-variant master playlist
	variantParams := buildVariantParams{
		startPosition:        req.StartPosition,
		strategy:             streamStrategy,
		supportedVideoCodecs: req.SupportedVideoCodecs,
		audioTrackIndex:      req.AudioTrackIndex,
	}
	playlist := uc.buildSingleVariantPlaylist(mediaItem, selectedQuality, audioTracks, subtitleTracks, variantParams, req.PreferredAudioLanguage, req.PreferredSubtitleLanguage)

	return &ServeMasterPlaylistResponse{
		Strategy:           StrategyServePlaylist,
		PlaylistContent:    playlist,
		Reason:             fmt.Sprintf("Single-variant playlist: %s (strategy: %s)", selectedQuality, streamStrategy),
		AvailableQualities: availableQualities,
	}, nil
}

// determineQuality selects the best quality based on client capabilities and source media.
// Priority: 1) User override, 2) Recommendation based on capabilities, 3) Fallback to source-matched quality
func (uc *ServeMasterPlaylistUseCase) determineQuality(req ServeMasterPlaylistRequest, mediaItem *media.Media) string {
	// Priority 1: User explicitly selected a quality
	if req.QualityOverride != "" {
		// Validate the override exists
		if _, ok := profile.GetABRVariant(req.QualityOverride); ok {
			return req.QualityOverride
		}
		// Invalid override, fall through to recommendation
	}

	// Priority 2: Use recommender if we have client capabilities
	if uc.recommender != nil && (req.ScreenWidth > 0 || req.ScreenHeight > 0 || req.Bandwidth > 0) {
		caps := profile.ClientCapabilities{
			ScreenWidth:      req.ScreenWidth,
			ScreenHeight:     req.ScreenHeight,
			NetworkSpeedMbps: float64(req.Bandwidth) / 1_000_000,
		}

		recommendation := uc.recommender.RecommendQuality(caps)
		recommendedQuality := recommendation.Profile.ID

		// Clamp to source resolution - don't recommend higher than source
		recommendedQuality = uc.clampToSource(recommendedQuality, mediaItem)

		return recommendedQuality
	}

	// Priority 3: Fallback - pick quality matching source resolution
	return uc.getDefaultQualityForSource(mediaItem)
}

// clampToSource ensures the recommended quality doesn't exceed source resolution/bitrate.
func (uc *ServeMasterPlaylistUseCase) clampToSource(qualityID string, mediaItem *media.Media) string {
	variant, ok := profile.GetABRVariant(qualityID)
	if !ok {
		return qualityID
	}

	// If recommended quality is higher resolution than source, find a matching one
	if variant.Height > mediaItem.Height {
		return uc.getDefaultQualityForSource(mediaItem)
	}

	// If recommended bitrate is higher than source (with tolerance), reduce
	if mediaItem.Bitrate > 0 && variant.Bandwidth > int(mediaItem.Bitrate*120/100) {
		return uc.getDefaultQualityForSource(mediaItem)
	}

	return qualityID
}

// getDefaultQualityForSource returns the best quality that matches the source.
func (uc *ServeMasterPlaylistUseCase) getDefaultQualityForSource(mediaItem *media.Media) string {
	// Find variants that match source resolution
	var bestMatch string
	var bestBandwidth int

	for _, variant := range profile.ABRLadder {
		// Skip if higher than source
		if variant.Height > mediaItem.Height {
			continue
		}

		// For matching height, pick highest bandwidth that doesn't exceed source
		if variant.Height == mediaItem.Height {
			if mediaItem.Bitrate > 0 && variant.Bandwidth > int(mediaItem.Bitrate*120/100) {
				continue
			}
			if variant.Bandwidth > bestBandwidth {
				bestBandwidth = variant.Bandwidth
				bestMatch = variant.ID
			}
		}
	}

	// If we found a match at source resolution, use it
	if bestMatch != "" {
		return bestMatch
	}

	// Fallback: find highest quality below source resolution
	for i := len(profile.ABRLadder) - 1; i >= 0; i-- {
		variant := profile.ABRLadder[i]
		if variant.Height <= mediaItem.Height {
			return variant.ID
		}
	}

	// Ultimate fallback
	return profile.Quality360p
}

// buildAvailableQualities returns all quality options valid for this media.
// Qualities are filtered to not exceed source resolution or bitrate.
func (uc *ServeMasterPlaylistUseCase) buildAvailableQualities(mediaItem *media.Media, selectedQuality string) []QualityOption {
	var qualities []QualityOption

	for _, variant := range profile.ABRLadder {
		// Skip qualities higher than source resolution
		if variant.Height > mediaItem.Height {
			continue
		}

		// Skip qualities with bitrate significantly higher than source (with 20% tolerance)
		if mediaItem.Bitrate > 0 && variant.Bandwidth > int(mediaItem.Bitrate*120/100) {
			continue
		}

		qualities = append(qualities, QualityOption{
			ID:          variant.ID,
			DisplayName: fmt.Sprintf("%dp (%d Mbps)", variant.Height, variant.Bandwidth/1_000_000),
			Width:       variant.Width,
			Height:      variant.Height,
			Bandwidth:   variant.Bandwidth,
			IsSelected:  variant.ID == selectedQuality,
		})
	}

	return qualities
}

// buildSingleVariantPlaylist creates an HLS master playlist with only one quality variant.
// This is the new default behavior - backend picks optimal quality, frontend plays it.
func (uc *ServeMasterPlaylistUseCase) buildSingleVariantPlaylist(_ *media.Media, qualityID string, audioTracks []*media.AudioTrack, subtitleTracks []*media.SubtitleTrack, params buildVariantParams, _ string, preferredSubtitleLang string) string {
	variant, ok := profile.GetABRVariant(qualityID)
	if !ok {
		// Fallback to a safe default if quality ID is invalid
		variant, _ = profile.GetABRVariant(profile.Quality720p4m)
	}

	// Build playlist header
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:4\n"
	playlist += "#EXT-X-INDEPENDENT-SEGMENTS\n\n"

	// Add subtitle track renditions for text-based subtitles
	subtitleGroupID := ""
	textSubtitles := uc.filterTextSubtitles(subtitleTracks)
	if len(textSubtitles) > 0 {
		subtitleGroupID = "subs"
		playlist += uc.buildSubtitleRenditions(textSubtitles, preferredSubtitleLang, params.startPosition)
		playlist += "\n"
	}

	// Suppress unused variable warning - audio tracks available for future use
	_ = audioTracks

	// Build single video stream variant
	streamInf := fmt.Sprintf(
		"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",NAME=\"%s\"",
		variant.Bandwidth,
		variant.Width,
		variant.Height,
		variant.Codecs,
		variant.ID,
	)

	// Reference subtitle group if we have text subtitles
	if subtitleGroupID != "" {
		streamInf += fmt.Sprintf(",SUBTITLES=\"%s\"", subtitleGroupID)
	}

	playlist += streamInf + "\n"

	// Build variant URL with query parameters
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
	if params.audioTrackIndex >= 0 {
		queryParams = append(queryParams, fmt.Sprintf("audioTrack=%d", params.audioTrackIndex))
	}
	if len(queryParams) > 0 {
		variantURL += "?" + strings.Join(queryParams, "&")
	}
	playlist += variantURL + "\n"

	return playlist
}

// buildVideoInfoFromDatabase creates a VideoInfo struct from database entities.
// This avoids the slow ffprobe call for network-mounted files since we already
// scanned and stored this information during library scanning.
// This is a package-level function so it can be used by both ServeManifestUseCase
// and ServeMasterPlaylistUseCase.
func buildVideoInfoFromDatabase(mediaItem *media.Media, audioTracks []*media.AudioTrack) *videoinfo.VideoInfo {
	info := &videoinfo.VideoInfo{
		VideoInfo: hls.VideoInfo{
			Codec:           mediaItem.VideoCodec,
			Width:           mediaItem.Width,
			Height:          mediaItem.Height,
			Bitrate:         mediaItem.Bitrate,
			ContainerFormat: mediaItem.ContainerFormat,
			ColorSpace:      mediaItem.ColorSpace,
			ColorPrimaries:  mediaItem.ColorPrimaries,
		},
		AudioTracks: make([]videoinfo.AudioTrack, 0, len(audioTracks)),
	}

	// Determine HDR status from stored HDRFormat or color metadata
	if mediaItem.HDRFormat != "" && mediaItem.HDRFormat != "SDR" {
		info.IsHDR = true
	} else if mediaItem.ColorPrimaries == "bt2020" {
		// BT.2020 with no explicit HDR format - check if it might be HDR
		// This is a conservative check; if in doubt, we'll transcode
		info.IsHDR = videoinfo.IsHDRContent("", mediaItem.ColorPrimaries)
	} else if mediaItem.HDRFormat == "" && mediaItem.ColorPrimaries == "" {
		// No HDR metadata in database - be conservative for 4K HEVC content
		// These are likely HDR files where the scanner didn't extract metadata
		codecLower := strings.ToLower(mediaItem.VideoCodec)
		isHEVC := codecLower == "hevc" || codecLower == "h265" || codecLower == "hev1"
		is4K := mediaItem.Width >= 3840 || mediaItem.Height >= 2160
		if isHEVC && is4K {
			// Assume 4K HEVC without color metadata might be HDR
			// This triggers full transcode with tone mapping to be safe
			info.IsHDR = true
		}
	}

	// Convert audio tracks from domain model to videoinfo format
	for _, at := range audioTracks {
		info.AudioTracks = append(info.AudioTracks, videoinfo.AudioTrack{
			Index:        at.StreamIndex,
			Codec:        at.Codec,
			Channels:     at.Channels,
			Bitrate:      int64(at.BitRate),
			Language:     at.Language,
			Title:        at.Title,
			IsCommentary: at.IsCommentary,
		})
	}

	// Select best audio track and populate main audio fields
	if bestTrack := videoinfo.SelectBestAudioTrack(info.AudioTracks); bestTrack != nil {
		info.AudioCodec = bestTrack.Codec
		info.AudioChannels = bestTrack.Channels
		info.AudioBitrate = bestTrack.Bitrate
		info.SelectedAudioTrackIndex = bestTrack.Index
	} else if len(info.AudioTracks) > 0 {
		// Fallback to first track
		info.AudioCodec = info.AudioTracks[0].Codec
		info.AudioChannels = info.AudioTracks[0].Channels
		info.AudioBitrate = info.AudioTracks[0].Bitrate
		info.SelectedAudioTrackIndex = info.AudioTracks[0].Index
	} else {
		// No audio tracks from database - use media's default audio codec
		// This handles cases where audio tracks weren't scanned separately
		info.AudioCodec = mediaItem.AudioCodec
	}

	return info
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

