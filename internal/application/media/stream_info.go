package media

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
)

// StreamInfoUseCase handles retrieving detailed stream information for a media file
type StreamInfoUseCase struct {
	mediaRepo   media.Repository
	libraryRepo library.Repository
}

// NewStreamInfoUseCase creates a new instance of StreamInfoUseCase
func NewStreamInfoUseCase(mediaRepo media.Repository, libraryRepo library.Repository) *StreamInfoUseCase {
	return &StreamInfoUseCase{
		mediaRepo:   mediaRepo,
		libraryRepo: libraryRepo,
	}
}

// AudioTrackInfo represents detailed audio track information
type AudioTrackInfo struct {
	Index        int    `json:"index"`
	Codec        string `json:"codec"`
	Channels     int    `json:"channels"`
	ChannelLabel string `json:"channel_label"` // "stereo", "5.1", "7.1", etc.
	Bitrate      int64  `json:"bitrate,omitempty"`
	Language     string `json:"language,omitempty"`
	Title        string `json:"title,omitempty"`
	IsDefault    bool   `json:"is_default"`
	IsCommentary bool   `json:"is_commentary"`
}

// VideoStreamInfo represents detailed video stream information
type VideoStreamInfo struct {
	Codec          string  `json:"codec"`
	Profile        string  `json:"profile,omitempty"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Bitrate        int64   `json:"bitrate,omitempty"`
	FrameRate      float64 `json:"frame_rate"`
	BitDepth       int     `json:"bit_depth"`
	IsHDR          bool    `json:"is_hdr"`
	HDRFormat      string  `json:"hdr_format,omitempty"` // "HDR10", "HLG", "Dolby Vision", etc.
	ColorSpace     string  `json:"color_space,omitempty"`
	ColorPrimaries string  `json:"color_primaries,omitempty"`
	PixelFormat    string  `json:"pixel_format,omitempty"`
}

// StreamingStrategy contains information about the playback strategy
type StreamingStrategy struct {
	Mode   string `json:"mode"`   // "direct", "remux", "remux_audio", "remux_hevc", "transcode"
	Reason string `json:"reason"` // Human-readable explanation
}

// ToneMappingInfo contains information about HDR to SDR tone mapping
type ToneMappingInfo struct {
	Enabled   bool   `json:"enabled"`             // Whether tone mapping is active for this stream
	Algorithm string `json:"algorithm,omitempty"` // Algorithm being used (bt.2390, hable, mobius, etc.)
	Backend   string `json:"backend,omitempty"`   // Backend being used (libplacebo, opencl, vaapi, cpu)
}

// StreamInfoResponse contains detailed stream information for the stats panel
type StreamInfoResponse struct {
	// Source file info
	Container string `json:"container"`
	FileSize  int64  `json:"file_size"`
	Duration  int    `json:"duration"`
	Bitrate   int64  `json:"bitrate"`

	// Video stream info
	Video VideoStreamInfo `json:"video"`

	// Audio tracks (all available)
	AudioTracks        []AudioTrackInfo `json:"audio_tracks"`
	SelectedAudioIndex int              `json:"selected_audio_index"`

	// Streaming strategy (what type of processing will be used)
	Strategy StreamingStrategy `json:"strategy"`

	// Tone mapping information (for HDR content)
	ToneMapping *ToneMappingInfo `json:"tone_mapping,omitempty"`
}

// Execute retrieves detailed stream information for a media file
func (uc *StreamInfoUseCase) Execute(ctx context.Context, mediaID int64) (*StreamInfoResponse, error) {
	// Get media record
	m, err := uc.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	// Determine full file path
	// If file_path is already absolute, use it directly
	// Otherwise, join with library path
	var fullPath string
	if filepath.IsAbs(m.FilePath) {
		fullPath = m.FilePath
	} else {
		lib, err := uc.libraryRepo.GetByID(ctx, m.LibraryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get library: %w", err)
		}
		fullPath = filepath.Join(lib.Path, m.FilePath)
	}

	// Get detailed video info using ffprobe (cached)
	videoInfo, err := videoinfo.GetVideoInfo(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// Build response
	response := &StreamInfoResponse{
		Container: normalizeContainer(videoInfo.ContainerFormat),
		FileSize:  m.FileSize,
		Duration:  m.Duration,
		Bitrate:   videoInfo.Bitrate,
		Video: VideoStreamInfo{
			Codec:          videoInfo.Codec,
			Width:          videoInfo.Width,
			Height:         videoInfo.Height,
			Bitrate:        videoInfo.Bitrate, // Video bitrate (overall for now)
			FrameRate:      m.FrameRate,
			BitDepth:       videoInfo.BitDepth,
			IsHDR:          videoInfo.IsHDR,
			HDRFormat:      getHDRFormat(videoInfo),
			ColorSpace:     videoInfo.ColorSpace,
			ColorPrimaries: videoInfo.ColorPrimaries,
			PixelFormat:    videoInfo.PixelFormat,
		},
		AudioTracks:        make([]AudioTrackInfo, 0, len(videoInfo.AudioTracks)),
		SelectedAudioIndex: videoInfo.SelectedAudioTrackIndex,
	}

	// Convert audio tracks
	for i, track := range videoInfo.AudioTracks {
		audioInfo := AudioTrackInfo{
			Index:        track.Index,
			Codec:        track.Codec,
			Channels:     track.Channels,
			ChannelLabel: getChannelLabel(track.Channels),
			Bitrate:      track.Bitrate,
			Language:     track.Language,
			Title:        track.Title,
			IsDefault:    i == 0 || track.Index == videoInfo.SelectedAudioTrackIndex,
			IsCommentary: track.IsCommentary,
		}
		response.AudioTracks = append(response.AudioTracks, audioInfo)
	}

	// Determine streaming strategy (default without client capabilities)
	// This gives a baseline estimate - the actual strategy is determined when playback starts
	// with client capability headers
	streamStrategy, reason := strategy.DetermineStrategy(videoInfo)
	response.Strategy = StreamingStrategy{
		Mode:   strategyToMode(streamStrategy),
		Reason: reason,
	}

	// Add tone mapping info for HDR content
	response.ToneMapping = getToneMappingInfo(videoInfo, streamStrategy)

	return response, nil
}

// ExecuteWithCapabilities retrieves stream information including strategy based on client capabilities and selected quality.
// The quality parameter allows accurate strategy detection - selecting a non-original quality on a remux source requires transcoding.
func (uc *StreamInfoUseCase) ExecuteWithCapabilities(ctx context.Context, mediaID int64, supportedVideoCodecs []string, supportsHDRDisplay bool, quality string) (*StreamInfoResponse, error) {
	// Get media record
	m, err := uc.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	// Determine full file path
	var fullPath string
	if filepath.IsAbs(m.FilePath) {
		fullPath = m.FilePath
	} else {
		lib, err := uc.libraryRepo.GetByID(ctx, m.LibraryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get library: %w", err)
		}
		fullPath = filepath.Join(lib.Path, m.FilePath)
	}

	// Get detailed video info using ffprobe (cached)
	videoInfo, err := videoinfo.GetVideoInfo(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// Build response
	response := &StreamInfoResponse{
		Container: normalizeContainer(videoInfo.ContainerFormat),
		FileSize:  m.FileSize,
		Duration:  m.Duration,
		Bitrate:   videoInfo.Bitrate,
		Video: VideoStreamInfo{
			Codec:          videoInfo.Codec,
			Width:          videoInfo.Width,
			Height:         videoInfo.Height,
			Bitrate:        videoInfo.Bitrate,
			FrameRate:      m.FrameRate,
			BitDepth:       videoInfo.BitDepth,
			IsHDR:          videoInfo.IsHDR,
			HDRFormat:      getHDRFormat(videoInfo),
			ColorSpace:     videoInfo.ColorSpace,
			ColorPrimaries: videoInfo.ColorPrimaries,
			PixelFormat:    videoInfo.PixelFormat,
		},
		AudioTracks:        make([]AudioTrackInfo, 0, len(videoInfo.AudioTracks)),
		SelectedAudioIndex: videoInfo.SelectedAudioTrackIndex,
	}

	// Convert audio tracks
	for i, track := range videoInfo.AudioTracks {
		audioInfo := AudioTrackInfo{
			Index:        track.Index,
			Codec:        track.Codec,
			Channels:     track.Channels,
			ChannelLabel: getChannelLabel(track.Channels),
			Bitrate:      track.Bitrate,
			Language:     track.Language,
			Title:        track.Title,
			IsDefault:    i == 0 || track.Index == videoInfo.SelectedAudioTrackIndex,
			IsCommentary: track.IsCommentary,
		}
		response.AudioTracks = append(response.AudioTracks, audioInfo)
	}

	// Determine streaming strategy with client capabilities
	var clientCaps *strategy.ClientCapabilities
	if len(supportedVideoCodecs) > 0 || supportsHDRDisplay {
		clientCaps = &strategy.ClientCapabilities{
			SupportedVideoCodecs: supportedVideoCodecs,
			SupportsHDRDisplay:   supportsHDRDisplay,
		}
	}
	streamStrategy, reason := strategy.DetermineStrategyWithCapabilities(videoInfo, clientCaps)

	// Check if non-original quality on remux source requires transcoding
	// Remux strategies copy video as-is, so selecting a different resolution requires transcoding
	isRemuxStrategy := streamStrategy == strategy.Remux ||
		streamStrategy == strategy.RemuxWithAudioDownmix ||
		streamStrategy == strategy.RemuxHEVC

	isOriginalQuality := quality == "" || quality == "original"

	if isRemuxStrategy && !isOriginalQuality {
		// User selected a specific quality profile on a remux source - must transcode
		streamStrategy = strategy.Transcode
		reason = fmt.Sprintf("transcoding required: user selected %s (remux only supports original)", quality)
	}

	response.Strategy = StreamingStrategy{
		Mode:   strategyToMode(streamStrategy),
		Reason: reason,
	}

	// Add tone mapping info for HDR content
	response.ToneMapping = getToneMappingInfo(videoInfo, streamStrategy)

	return response, nil
}

// strategyToMode converts internal StreamStrategy to frontend-friendly mode string
func strategyToMode(s strategy.StreamStrategy) string {
	switch s {
	case strategy.DirectPlay:
		return "direct"
	case strategy.Remux:
		return "remux"
	case strategy.RemuxWithAudioDownmix:
		return "remux_audio"
	case strategy.RemuxHEVC:
		return "remux_hevc"
	case strategy.Transcode:
		return "transcode"
	default:
		return "transcode"
	}
}

// normalizeContainer converts ffprobe format names to user-friendly names
func normalizeContainer(format string) string {
	// ffprobe returns multiple formats like "matroska,webm"
	// We want to show the primary container name
	switch {
	case contains(format, "matroska"):
		return "MKV"
	case contains(format, "mp4"):
		return "MP4"
	case contains(format, "avi"):
		return "AVI"
	case contains(format, "mov"):
		return "MOV"
	case contains(format, "webm"):
		return "WebM"
	case contains(format, "mpegts"):
		return "MPEG-TS"
	case contains(format, "mpeg"):
		return "MPEG"
	default:
		return format
	}
}

// getChannelLabel converts channel count to human-readable label
func getChannelLabel(channels int) string {
	switch channels {
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		return fmt.Sprintf("%d ch", channels)
	}
}

// getHDRFormat determines the HDR format name from video info
func getHDRFormat(info *videoinfo.VideoInfo) string {
	if !info.IsHDR {
		return ""
	}

	switch info.ColorTransfer {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	default:
		if info.ColorPrimaries == "bt2020" {
			return "HDR"
		}
		return ""
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// getToneMappingInfo returns tone mapping information for HDR content.
// Returns nil if content is not HDR or doesn't require tone mapping.
func getToneMappingInfo(videoInfo *videoinfo.VideoInfo, s strategy.StreamStrategy) *ToneMappingInfo {
	// Only applicable for HDR content being transcoded
	if videoInfo == nil || !videoInfo.IsHDR {
		return nil
	}

	// Only transcode strategy performs tone mapping
	// Direct play, remux modes don't process video
	if s != strategy.Transcode {
		return nil
	}

	// Get current tone mapping config from defaults (reads env vars)
	cfg := config.Default()

	if !cfg.ToneMappingEnabled {
		return &ToneMappingInfo{
			Enabled: false,
		}
	}

	// Determine the backend being used
	backend := cfg.ToneMappingBackend
	if backend == "" || backend == "auto" {
		// Auto mode: determine based on hardware acceleration
		// NVENC uses libplacebo, VAAPI uses native, others use OpenCL/CPU
		switch cfg.HardwareAccel {
		case hls.AccelNVENC:
			if hls.CheckFFmpegFilter("libplacebo") {
				backend = "libplacebo"
			} else {
				backend = "opencl"
			}
		case hls.AccelVAAPI:
			backend = "vaapi"
		case hls.AccelNone:
			if hls.CheckFFmpegFilter("libplacebo") {
				backend = "libplacebo"
			} else {
				backend = "cpu"
			}
		default:
			backend = "opencl"
		}
	}

	return &ToneMappingInfo{
		Enabled:   true,
		Algorithm: cfg.ToneMappingAlgorithm,
		Backend:   backend,
	}
}
