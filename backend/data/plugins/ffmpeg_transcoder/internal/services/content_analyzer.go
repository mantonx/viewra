package services

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/pkg/plugins"
)

// ContentType represents the type of content being transcoded
type ContentType string

const (
	ContentTypeMovie   ContentType = "movie"
	ContentTypeTVShow  ContentType = "tv_show"
	ContentTypeUnknown ContentType = "unknown"
)

// ContentQuality represents the quality level of the source content
type ContentQuality string

const (
	ContentQualityRemux    ContentQuality = "remux"    // Full quality, theatrical releases
	ContentQualityWebDL    ContentQuality = "webdl"    // High quality web downloads
	ContentQualityBluray   ContentQuality = "bluray"   // Blu-ray rips
	ContentQualityStandard ContentQuality = "standard" // Standard quality
	ContentQualityLow      ContentQuality = "low"      // Low quality sources
)

// VideoCharacteristics contains detailed information about the source video
type VideoCharacteristics struct {
	// Basic properties
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FrameRate float64 `json:"frame_rate"`
	Duration  float64 `json:"duration"`
	Bitrate   int64   `json:"bitrate"`
	FileSize  int64   `json:"file_size"`

	// Video codec info
	VideoCodec     string `json:"video_codec"`
	VideoProfile   string `json:"video_profile"`
	PixelFormat    string `json:"pixel_format"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	BitDepth       int    `json:"bit_depth"`

	// HDR information
	IsHDR         bool   `json:"is_hdr"`
	IsDolbyVision bool   `json:"is_dolby_vision"`
	HDRType       string `json:"hdr_type"`

	// Audio information
	AudioCodec      string `json:"audio_codec"`
	AudioChannels   int    `json:"audio_channels"`
	AudioBitrate    int    `json:"audio_bitrate"`
	AudioSampleRate int    `json:"audio_sample_rate"`

	// Content classification
	ContentType    ContentType    `json:"content_type"`
	ContentQuality ContentQuality `json:"content_quality"`
	IsRemux        bool           `json:"is_remux"`
	IsWebDL        bool           `json:"is_webdl"`

	// Subtitle information
	HasSubtitles   bool `json:"has_subtitles"`
	SubtitleTracks int  `json:"subtitle_tracks"`
}

// TranscodingProfile contains the optimal settings for transcoding
type TranscodingProfile struct {
	// Basic settings
	TargetResolution string `json:"target_resolution"`
	TargetBitrate    int    `json:"target_bitrate"`
	MaxBitrate       int    `json:"max_bitrate"`
	BufferSize       int    `json:"buffer_size"`

	// Codec settings
	VideoCodec string  `json:"video_codec"`
	CRF        float64 `json:"crf"`
	Preset     string  `json:"preset"`
	Tune       string  `json:"tune"`
	TwoPass    bool    `json:"two_pass"`

	// Audio settings
	AudioCodec    string `json:"audio_codec"`
	AudioBitrate  int    `json:"audio_bitrate"`
	AudioChannels int    `json:"audio_channels"`

	// HDR handling
	HDRHandling string `json:"hdr_handling"` // preserve, tonemap, none
	ToneMapping bool   `json:"tone_mapping"`

	// Filtering
	UseDenoising     bool `json:"use_denoising"`
	UseSharpening    bool `json:"use_sharpening"`
	UseDeinterlacing bool `json:"use_deinterlacing"`

	// Quality vs speed optimization
	QualityOptimized bool `json:"quality_optimized"`
	SpeedOptimized   bool `json:"speed_optimized"`
}

// ContentAnalyzer analyzes video content and selects optimal transcoding profiles
type ContentAnalyzer struct {
	logger        plugins.Logger
	configService *config.FFmpegConfigurationService
}

// NewContentAnalyzer creates a new content analyzer
func NewContentAnalyzer(logger plugins.Logger, configService *config.FFmpegConfigurationService) *ContentAnalyzer {
	return &ContentAnalyzer{
		logger:        logger,
		configService: configService,
	}
}

// AnalyzeContent analyzes a video file and returns its characteristics
func (ca *ContentAnalyzer) AnalyzeContent(filePath string) (*VideoCharacteristics, error) {
	ca.logger.Debug("analyzing content", "file", filePath)

	// Use ffprobe to get detailed video information
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to analyze content with ffprobe: %w", err)
	}

	var probe struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
			Size     string `json:"size"`
		} `json:"format"`
		Streams []struct {
			Index          int    `json:"index"`
			CodecType      string `json:"codec_type"`
			CodecName      string `json:"codec_name"`
			Profile        string `json:"profile"`
			Width          int    `json:"width"`
			Height         int    `json:"height"`
			PixFmt         string `json:"pix_fmt"`
			ColorSpace     string `json:"color_space"`
			ColorTransfer  string `json:"color_transfer"`
			ColorPrimaries string `json:"color_primaries"`
			RFrameRate     string `json:"r_frame_rate"`
			BitRate        string `json:"bit_rate"`
			SampleRate     string `json:"sample_rate"`
			Channels       int    `json:"channels"`
			BitsPerSample  int    `json:"bits_per_raw_sample"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	characteristics := &VideoCharacteristics{}

	// Parse format information
	if probe.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			characteristics.Duration = duration
		}
	}

	if probe.Format.BitRate != "" {
		if bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
			characteristics.Bitrate = bitrate
		}
	}

	if probe.Format.Size != "" {
		if size, err := strconv.ParseInt(probe.Format.Size, 10, 64); err == nil {
			characteristics.FileSize = size
		}
	}

	// Analyze video and audio streams
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			ca.analyzeVideoStream(stream, characteristics)
		case "audio":
			ca.analyzeAudioStream(stream, characteristics)
		case "subtitle":
			characteristics.HasSubtitles = true
			characteristics.SubtitleTracks++
		}
	}

	// Analyze filename for content classification
	ca.analyzeFilename(filePath, characteristics)

	// Detect HDR characteristics
	ca.detectHDR(characteristics)

	ca.logger.Info("content analysis complete",
		"resolution", fmt.Sprintf("%dx%d", characteristics.Width, characteristics.Height),
		"codec", characteristics.VideoCodec,
		"hdr", characteristics.IsHDR,
		"content_type", characteristics.ContentType,
		"quality", characteristics.ContentQuality,
	)

	return characteristics, nil
}

// SelectOptimalProfile selects the best transcoding profile based on content and target device
func (ca *ContentAnalyzer) SelectOptimalProfile(characteristics *VideoCharacteristics, targetDevice string, targetBandwidth int) (*TranscodingProfile, error) {
	profile := &TranscodingProfile{}

	// Determine target resolution based on source and bandwidth
	profile.TargetResolution = ca.selectTargetResolution(characteristics, targetBandwidth)

	// Select optimal codec based on content and device
	profile.VideoCodec = ca.selectVideoCodec(characteristics, targetDevice)

	// Determine quality settings
	profile.CRF = ca.selectCRF(characteristics, profile.VideoCodec)
	profile.Preset = ca.selectPreset(characteristics)
	profile.Tune = ca.selectTune(characteristics)

	// Configure bitrate settings
	ca.configureBitrate(profile, characteristics)

	// Configure audio settings
	ca.configureAudio(profile, characteristics, targetDevice)

	// Configure HDR handling
	ca.configureHDR(profile, characteristics, targetDevice)

	// Configure filtering based on content quality
	ca.configureFiltering(profile, characteristics)

	// Optimize for content type (movie vs TV)
	ca.optimizeForContentType(profile, characteristics)

	ca.logger.Info("selected transcoding profile",
		"target_resolution", profile.TargetResolution,
		"video_codec", profile.VideoCodec,
		"crf", profile.CRF,
		"preset", profile.Preset,
		"target_bitrate", profile.TargetBitrate,
		"hdr_handling", profile.HDRHandling,
	)

	return profile, nil
}

// analyzeVideoStream extracts video stream information
func (ca *ContentAnalyzer) analyzeVideoStream(stream struct {
	Index          int    `json:"index"`
	CodecType      string `json:"codec_type"`
	CodecName      string `json:"codec_name"`
	Profile        string `json:"profile"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	PixFmt         string `json:"pix_fmt"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	RFrameRate     string `json:"r_frame_rate"`
	BitRate        string `json:"bit_rate"`
	SampleRate     string `json:"sample_rate"`
	Channels       int    `json:"channels"`
	BitsPerSample  int    `json:"bits_per_raw_sample"`
}, characteristics *VideoCharacteristics) {
	characteristics.Width = stream.Width
	characteristics.Height = stream.Height
	characteristics.VideoCodec = stream.CodecName
	characteristics.VideoProfile = stream.Profile
	characteristics.PixelFormat = stream.PixFmt
	characteristics.ColorSpace = stream.ColorSpace
	characteristics.ColorTransfer = stream.ColorTransfer
	characteristics.ColorPrimaries = stream.ColorPrimaries

	// Parse frame rate
	if stream.RFrameRate != "" {
		parts := strings.Split(stream.RFrameRate, "/")
		if len(parts) == 2 {
			if num, err := strconv.ParseFloat(parts[0], 64); err == nil {
				if den, err := strconv.ParseFloat(parts[1], 64); err == nil && den > 0 {
					characteristics.FrameRate = num / den
				}
			}
		}
	}

	// Determine bit depth
	if strings.Contains(stream.PixFmt, "10") {
		characteristics.BitDepth = 10
	} else {
		characteristics.BitDepth = 8
	}
}

// analyzeAudioStream extracts audio stream information
func (ca *ContentAnalyzer) analyzeAudioStream(stream struct {
	Index          int    `json:"index"`
	CodecType      string `json:"codec_type"`
	CodecName      string `json:"codec_name"`
	Profile        string `json:"profile"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	PixFmt         string `json:"pix_fmt"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	RFrameRate     string `json:"r_frame_rate"`
	BitRate        string `json:"bit_rate"`
	SampleRate     string `json:"sample_rate"`
	Channels       int    `json:"channels"`
	BitsPerSample  int    `json:"bits_per_raw_sample"`
}, characteristics *VideoCharacteristics) {
	// Only process first audio stream for primary characteristics
	if characteristics.AudioCodec == "" {
		characteristics.AudioCodec = stream.CodecName
		characteristics.AudioChannels = stream.Channels

		if stream.SampleRate != "" {
			if rate, err := strconv.Atoi(stream.SampleRate); err == nil {
				characteristics.AudioSampleRate = rate
			}
		}

		if stream.BitRate != "" {
			if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
				characteristics.AudioBitrate = bitrate / 1000 // Convert to kbps
			}
		}
	}
}

// analyzeFilename extracts information from the filename
func (ca *ContentAnalyzer) analyzeFilename(filePath string, characteristics *VideoCharacteristics) {
	filename := strings.ToLower(filepath.Base(filePath))

	// Detect content type
	tvIndicators := []string{"s01e", "s02e", "s03e", "s04e", "s05e", "season", "episode"}
	for _, indicator := range tvIndicators {
		if strings.Contains(filename, indicator) {
			characteristics.ContentType = ContentTypeTVShow
			break
		}
	}
	if characteristics.ContentType == "" {
		characteristics.ContentType = ContentTypeMovie
	}

	// Detect content quality
	if strings.Contains(filename, "remux") {
		characteristics.ContentQuality = ContentQualityRemux
		characteristics.IsRemux = true
	} else if strings.Contains(filename, "webdl") || strings.Contains(filename, "web-dl") || strings.Contains(filename, "webrip") {
		characteristics.ContentQuality = ContentQualityWebDL
		characteristics.IsWebDL = true
	} else if strings.Contains(filename, "bluray") || strings.Contains(filename, "blu-ray") {
		characteristics.ContentQuality = ContentQualityBluray
	} else if characteristics.Bitrate > 15000000 { // > 15 Mbps
		characteristics.ContentQuality = ContentQualityStandard
	} else {
		characteristics.ContentQuality = ContentQualityLow
	}
}

// detectHDR determines if content is HDR and what type
func (ca *ContentAnalyzer) detectHDR(characteristics *VideoCharacteristics) {
	// Check color transfer for HDR indicators
	if characteristics.ColorTransfer == "smpte2084" || characteristics.ColorTransfer == "arib-std-b67" {
		characteristics.IsHDR = true
		if characteristics.ColorTransfer == "smpte2084" {
			characteristics.HDRType = "HDR10"
		} else {
			characteristics.HDRType = "HLG"
		}
	}

	// Check color space for bt2020
	if characteristics.ColorSpace == "bt2020nc" || characteristics.ColorSpace == "bt2020c" {
		characteristics.IsHDR = true
		if characteristics.HDRType == "" {
			characteristics.HDRType = "HDR10"
		}
	}

	// Check for Dolby Vision (this would need more sophisticated detection)
	// For now, assume DV if we have HDR with 10-bit depth and HEVC
	if characteristics.IsHDR && characteristics.BitDepth == 10 && characteristics.VideoCodec == "hevc" {
		// Additional DV detection could be added here
	}
}

// selectTargetResolution determines the best target resolution
func (ca *ContentAnalyzer) selectTargetResolution(characteristics *VideoCharacteristics, targetBandwidth int) string {
	// Don't upscale - target resolution should not exceed source
	if characteristics.Height >= 2160 && targetBandwidth >= 20000 {
		return "2160p"
	} else if characteristics.Height >= 1440 && targetBandwidth >= 10000 {
		return "1440p"
	} else if characteristics.Height >= 1080 && targetBandwidth >= 5000 {
		return "1080p"
	} else if characteristics.Height >= 720 && targetBandwidth >= 2000 {
		return "720p"
	} else {
		return "480p"
	}
}

// selectVideoCodec chooses the best video codec for the target device
func (ca *ContentAnalyzer) selectVideoCodec(characteristics *VideoCharacteristics, targetDevice string) string {
	switch targetDevice {
	case "web_modern":
		if characteristics.IsHDR {
			return "hevc" // Better HDR support
		}
		return "h264" // Widest compatibility
	case "web_legacy":
		return "h264"
	case "roku", "apple_tv":
		return "hevc" // Good HEVC support
	case "nvidia_shield", "android_tv":
		if characteristics.Width >= 3840 { // 4K content
			return "hevc" // Better efficiency for 4K
		}
		return "h264"
	default:
		return "h264" // Safe default
	}
}

// selectCRF chooses the optimal CRF value
func (ca *ContentAnalyzer) selectCRF(characteristics *VideoCharacteristics, codec string) float64 {
	baseCRF := map[string]float64{
		"h264": 23.0,
		"hevc": 28.0,
		"av1":  32.0,
	}

	crf := baseCRF[codec]

	// Adjust based on content quality
	switch characteristics.ContentQuality {
	case ContentQualityRemux:
		crf -= 2.0 // Higher quality for remux sources
	case ContentQualityWebDL:
		crf -= 1.0 // Slightly higher quality
	case ContentQualityLow:
		crf += 1.0 // Lower quality for poor sources
	}

	// Adjust for content type
	if characteristics.ContentType == ContentTypeMovie {
		crf -= 1.0 // Movies get slightly higher quality
	}

	// Ensure reasonable bounds
	if crf < 15.0 {
		crf = 15.0
	} else if crf > 35.0 {
		crf = 35.0
	}

	return crf
}

// selectPreset chooses the optimal encoding preset
func (ca *ContentAnalyzer) selectPreset(characteristics *VideoCharacteristics) string {
	// Favor quality for movies and high-quality content
	if characteristics.ContentType == ContentTypeMovie && characteristics.ContentQuality == ContentQualityRemux {
		return "slow"
	} else if characteristics.ContentQuality == ContentQualityRemux || characteristics.ContentQuality == ContentQualityBluray {
		return "medium"
	} else {
		return "fast" // Good balance for most content
	}
}

// selectTune chooses the optimal tune setting
func (ca *ContentAnalyzer) selectTune(characteristics *VideoCharacteristics) string {
	if characteristics.ContentType == ContentTypeMovie {
		return "film"
	} else {
		return "film" // Film works well for most content
	}
}

// configureBitrate sets up bitrate-related settings
func (ca *ContentAnalyzer) configureBitrate(profile *TranscodingProfile, characteristics *VideoCharacteristics) {
	// Base bitrate from resolution
	baseBitrates := map[string]int{
		"480p":  1500,
		"720p":  3000,
		"1080p": 6000,
		"1440p": 12000,
		"2160p": 25000,
	}

	profile.TargetBitrate = baseBitrates[profile.TargetResolution]

	// Adjust for content quality and type
	multiplier := 1.0
	if characteristics.ContentQuality == ContentQualityRemux {
		multiplier = 1.3
	} else if characteristics.ContentType == ContentTypeMovie {
		multiplier = 1.2
	}

	// Apply HDR adjustment
	if characteristics.IsHDR {
		multiplier *= 1.2
	}

	profile.TargetBitrate = int(float64(profile.TargetBitrate) * multiplier)
	profile.MaxBitrate = int(float64(profile.TargetBitrate) * 1.5)
	profile.BufferSize = profile.MaxBitrate * 2
}

// configureAudio sets up audio encoding settings
func (ca *ContentAnalyzer) configureAudio(profile *TranscodingProfile, characteristics *VideoCharacteristics, targetDevice string) {
	// Default to AAC for compatibility
	profile.AudioCodec = "aac"
	profile.AudioChannels = characteristics.AudioChannels

	// Limit channels for compatibility
	if profile.AudioChannels > 6 {
		profile.AudioChannels = 6 // 5.1 max for most devices
	}

	// Set bitrate based on channel count and quality
	if profile.AudioChannels >= 6 {
		profile.AudioBitrate = 256 // 5.1+ audio
	} else if profile.AudioChannels > 2 {
		profile.AudioBitrate = 192 // Surround audio
	} else {
		profile.AudioBitrate = 128 // Stereo audio
	}

	// Use higher quality audio for premium content
	if characteristics.ContentQuality == ContentQualityRemux {
		profile.AudioBitrate = int(float64(profile.AudioBitrate) * 1.5)
	}
}

// configureHDR sets up HDR handling
func (ca *ContentAnalyzer) configureHDR(profile *TranscodingProfile, characteristics *VideoCharacteristics, targetDevice string) {
	if !characteristics.IsHDR {
		profile.HDRHandling = "none"
		return
	}

	// Check device HDR support
	switch targetDevice {
	case "web_modern", "roku", "nvidia_shield", "apple_tv", "android_tv":
		profile.HDRHandling = "preserve"
	case "web_legacy":
		profile.HDRHandling = "tonemap"
		profile.ToneMapping = true
	default:
		profile.HDRHandling = "tonemap"
		profile.ToneMapping = true
	}
}

// configureFiltering sets up video filtering
func (ca *ContentAnalyzer) configureFiltering(profile *TranscodingProfile, characteristics *VideoCharacteristics) {
	// Enable denoising for lower quality sources
	if characteristics.ContentQuality == ContentQualityLow {
		profile.UseDenoising = true
	}

	// Enable sharpening if upscaling significantly
	targetHeight := map[string]int{
		"480p": 480, "720p": 720, "1080p": 1080, "1440p": 1440, "2160p": 2160,
	}
	if targetHeight[profile.TargetResolution] > int(float64(characteristics.Height)*1.5) {
		profile.UseSharpening = true
	}

	// Always enable deinterlacing detection
	profile.UseDeinterlacing = true
}

// optimizeForContentType applies content-specific optimizations
func (ca *ContentAnalyzer) optimizeForContentType(profile *TranscodingProfile, characteristics *VideoCharacteristics) {
	if characteristics.ContentType == ContentTypeMovie {
		// Movies prioritize quality
		profile.QualityOptimized = true
		profile.TwoPass = characteristics.ContentQuality == ContentQualityRemux
	} else {
		// TV shows prioritize speed for batch processing
		profile.SpeedOptimized = true
	}
}
