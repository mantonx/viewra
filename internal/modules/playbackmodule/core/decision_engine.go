// Package core provides the core functionality for the playback module.
package core

import (
	"context"
	"fmt"

	"strings"

	"github.com/hashicorp/go-hclog"
	playbacktypes "github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	playbackutils "github.com/mantonx/viewra/internal/modules/playbackmodule/utils"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)


// DecisionEngine determines the best playback method for media files
// based on device capabilities and file characteristics.
type DecisionEngine struct {
	logger         hclog.Logger
	mediaService   services.MediaService
	deviceDetector *DeviceDetector
}

// NewDecisionEngine creates a new playback decision engine
func NewDecisionEngine(logger hclog.Logger, mediaService services.MediaService) *DecisionEngine {
	return &DecisionEngine{
		logger:         logger,
		mediaService:   mediaService,
		deviceDetector: NewDeviceDetector(logger.Named("device-detector")),
	}
}

// DecidePlayback analyzes a media file and device profile to determine
// the optimal playback method.
func (de *DecisionEngine) DecidePlayback(ctx context.Context, mediaPath string, deviceProfile *playbacktypes.DeviceProfile) (*playbacktypes.PlaybackDecision, error) {
	// Enhance device profile with device capabilities if user agent is provided
	if deviceProfile.UserAgent != "" {
		de.enhanceProfileWithDeviceCaps(deviceProfile)
	}

	de.logger.Info("Deciding playback method", "path", mediaPath, "device", deviceProfile.Name,
		"supportedVideoCodecs", deviceProfile.SupportedVideoCodecs,
		"supportedAudioCodecs", deviceProfile.SupportedAudioCodecs,
		"supportedContainers", deviceProfile.SupportedContainers)

	// Get media information
	mediaInfo, err := de.mediaService.GetMediaInfo(ctx, mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get media info: %w", err)
	}

	// Make decision based on compatibility
	method := de.determineMethod(mediaInfo, deviceProfile)

	decision := &playbacktypes.PlaybackDecision{
		Method:     method,
		MediaPath:  mediaPath,
		Reason:     de.getDecisionReason(method, mediaInfo, deviceProfile),
		MediaInfo:  mediaInfo,
		DeviceInfo: deviceProfile,
	}

	// If transcoding is needed, prepare transcode parameters
	if method == types.PlaybackMethodTranscode {
		decision.TranscodeParams = de.getTranscodeParams(mediaInfo, deviceProfile)
	}

	de.logger.Info("Playback decision made",
		"method", method,
		"reason", decision.Reason)

	return decision, nil
}

// determineMethod determines the playback method based on compatibility
func (de *DecisionEngine) determineMethod(mediaInfo *types.MediaInfo, profile *playbacktypes.DeviceProfile) types.PlaybackMethod {
	// Check if this is an audio-only file
	isAudioOnly := de.isAudioOnlyFile(mediaInfo)

	// Check container compatibility
	containerSupported := de.isContainerSupported(mediaInfo.Container, profile)

	// Check video codec compatibility (only if video streams exist)
	videoSupported := true
	if !isAudioOnly && len(mediaInfo.VideoStreams) > 0 {
		videoSupported = de.isVideoCodecSupported(mediaInfo.VideoStreams[0].Codec, profile)
	}

	// Check audio codec compatibility
	audioSupported := true
	if len(mediaInfo.AudioStreams) > 0 {
		audioSupported = de.isAudioCodecSupported(mediaInfo.AudioStreams[0].Codec, profile)
	}

	// Check resolution limits (only for video files)
	resolutionSupported := true
	if !isAudioOnly && len(mediaInfo.VideoStreams) > 0 && profile.MaxResolution != "" {
		resolutionSupported = de.isResolutionSupported(
			mediaInfo.VideoStreams[0].Width,
			mediaInfo.VideoStreams[0].Height,
			profile.MaxResolution,
		)
	}

	// Check bitrate limits
	bitrateSupported := true
	if profile.MaxBitrate > 0 && mediaInfo.Bitrate > 0 {
		bitrateSupported = mediaInfo.Bitrate <= int64(profile.MaxBitrate)
	}

	// Decision logic
	if containerSupported && videoSupported && audioSupported && resolutionSupported && bitrateSupported {
		return types.PlaybackMethodDirect
	}

	// If only container is incompatible, we can remux
	if !containerSupported && videoSupported && audioSupported && resolutionSupported && bitrateSupported {
		// For audio-only files, prefer transcoding over remuxing for better compatibility
		if isAudioOnly && de.shouldTranscodeAudio(mediaInfo, profile) {
			return types.PlaybackMethodTranscode
		}
		return types.PlaybackMethodRemux
	}

	// Otherwise, full transcode is needed
	return types.PlaybackMethodTranscode
}

// isContainerSupported checks if a container format is supported
func (de *DecisionEngine) isContainerSupported(container string, profile *playbacktypes.DeviceProfile) bool {
	container = strings.ToLower(container)
	
	// Handle container aliases
	containerAliases := map[string][]string{
		"matroska": {"mkv"},
		"mkv":      {"matroska"},
		"mpeg4":    {"mp4"},
		"mp4":      {"mpeg4"},
		"webm":     {"webm"},
		"ogg":      {"oga", "ogv"},
	}
	
	for _, supported := range profile.SupportedContainers {
		supportedLower := strings.ToLower(supported)
		// Direct match
		if supportedLower == container {
			return true
		}
		// Check aliases
		if aliases, exists := containerAliases[supportedLower]; exists {
			for _, alias := range aliases {
				if alias == container {
					return true
				}
			}
		}
	}
	return false
}

// isVideoCodecSupported checks if a video codec is supported
func (de *DecisionEngine) isVideoCodecSupported(codec string, profile *playbacktypes.DeviceProfile) bool {
	codec = strings.ToLower(codec)
	for _, supported := range profile.SupportedVideoCodecs {
		if strings.ToLower(supported) == codec {
			return true
		}
	}
	return false
}

// isAudioCodecSupported checks if an audio codec is supported
func (de *DecisionEngine) isAudioCodecSupported(codec string, profile *playbacktypes.DeviceProfile) bool {
	codec = strings.ToLower(codec)
	for _, supported := range profile.SupportedAudioCodecs {
		if strings.ToLower(supported) == codec {
			return true
		}
	}
	return false
}

// isResolutionSupported checks if the resolution is within device limits
func (de *DecisionEngine) isResolutionSupported(width, height int, maxResolution string) bool {
	// Parse max resolution (e.g., "1080p", "4K")
	maxHeight := 0
	switch strings.ToLower(maxResolution) {
	case "480p":
		maxHeight = 480
	case "720p":
		maxHeight = 720
	case "1080p":
		maxHeight = 1080
	case "4k", "2160p":
		maxHeight = 2160
	default:
		// If we can't parse, assume it's supported
		return true
	}

	return height <= maxHeight
}

// getDecisionReason provides a human-readable reason for the decision
func (de *DecisionEngine) getDecisionReason(method types.PlaybackMethod, mediaInfo *types.MediaInfo, profile *playbacktypes.DeviceProfile) string {
	switch method {
	case types.PlaybackMethodDirect:
		return "File is fully compatible with device"
	case types.PlaybackMethodRemux:
		return fmt.Sprintf("Container format '%s' not supported, remuxing to %s",
			mediaInfo.Container, profile.PreferredContainer)
	case types.PlaybackMethodTranscode:
		reasons := []string{}

		if len(mediaInfo.VideoStreams) > 0 && !de.isVideoCodecSupported(mediaInfo.VideoStreams[0].Codec, profile) {
			reasons = append(reasons, fmt.Sprintf("video codec '%s' not supported", mediaInfo.VideoStreams[0].Codec))
		}

		if len(mediaInfo.AudioStreams) > 0 && !de.isAudioCodecSupported(mediaInfo.AudioStreams[0].Codec, profile) {
			reasons = append(reasons, fmt.Sprintf("audio codec '%s' not supported", mediaInfo.AudioStreams[0].Codec))
		}

		if len(reasons) > 0 {
			return "Transcoding required: " + strings.Join(reasons, ", ")
		}

		return "Transcoding required for device compatibility"
	default:
		return "Unknown playback method"
	}
}

// getTranscodeParams generates transcoding parameters based on device profile
func (de *DecisionEngine) getTranscodeParams(mediaInfo *types.MediaInfo, profile *playbacktypes.DeviceProfile) *plugins.TranscodeRequest {
	params := &plugins.TranscodeRequest{
		Container:  profile.PreferredContainer,
		VideoCodec: profile.PreferredVideoCodec,
		AudioCodec: profile.PreferredAudioCodec,
	}

	// For audio-only files, clear video codec and use audio-specific container
	if de.isAudioOnlyFile(mediaInfo) {
		params.VideoCodec = ""
		// Prefer M4A for AAC, OGG for Opus/Vorbis, etc.
		if profile.PreferredAudioCodec == "aac" {
			params.Container = "m4a"
		} else if profile.PreferredAudioCodec == "opus" || profile.PreferredAudioCodec == "vorbis" {
			params.Container = "ogg"
		}
	}

	// Set resolution based on device limits
	if profile.MaxResolution != "" && len(mediaInfo.VideoStreams) > 0 {
		switch profile.MaxResolution {
		case "720p":
			if mediaInfo.VideoStreams[0].Height > 720 {
				params.Resolution = &plugins.Resolution{
					Width:  1280,
					Height: 720,
				}
			}
		case "1080p":
			if mediaInfo.VideoStreams[0].Height > 1080 {
				params.Resolution = &plugins.Resolution{
					Width:  1920,
					Height: 1080,
				}
			}
		}
	}

	// Set bitrate limits
	if profile.MaxBitrate > 0 {
		// Use 80% of max bitrate for video, 20% for audio
		params.VideoBitrate = int(float64(profile.MaxBitrate) * 0.8)
		params.AudioBitrate = int(float64(profile.MaxBitrate) * 0.2)
	}

	return params
}

// GetSupportedFormats returns formats that can be directly played on a device
func (de *DecisionEngine) GetSupportedFormats(profile *playbacktypes.DeviceProfile) []string {
	formats := []string{}

	// Add container formats
	for _, container := range profile.SupportedContainers {
		formats = append(formats, strings.ToUpper(container))
	}

	return formats
}

// ValidatePlayback checks if a media file can be played on a device
func (de *DecisionEngine) ValidatePlayback(ctx context.Context, mediaPath string, profile *playbacktypes.DeviceProfile) error {
	decision, err := de.DecidePlayback(ctx, mediaPath, profile)
	if err != nil {
		return fmt.Errorf("failed to make playback decision: %w", err)
	}

	// Any method is valid - we can always play the file somehow
	de.logger.Debug("Playback validated",
		"path", mediaPath,
		"method", decision.Method,
		"device", profile.Name)

	return nil
}

// enhanceProfileWithDeviceCaps updates device profile based on device detection
func (de *DecisionEngine) enhanceProfileWithDeviceCaps(profile *playbacktypes.DeviceProfile) {
	caps := de.deviceDetector.DetectCapabilities(profile.UserAgent)

	// Get codec lists from device capabilities
	videoCodecs, audioCodecs, containers := de.deviceDetector.ConvertToDeviceProfile(caps)

	// Only enhance with browser detection if frontend didn't provide detailed codec support
	if len(profile.SupportedVideoCodecs) == 0 && len(videoCodecs) > 0 {
		profile.SupportedVideoCodecs = videoCodecs
	}
	if len(profile.SupportedAudioCodecs) == 0 && len(audioCodecs) > 0 {
		profile.SupportedAudioCodecs = audioCodecs
	}
	if len(profile.SupportedContainers) == 0 && len(containers) > 0 {
		profile.SupportedContainers = containers
	}

	// Update max resolution based on browser capabilities
	if caps.MaxResolution != "" {
		profile.MaxResolution = caps.MaxResolution
	}

	// Set browser-specific preferences
	if caps.WebM && caps.VP9 {
		// Prefer WebM for browsers with good support
		profile.PreferredContainer = "webm"
		profile.PreferredVideoCodec = "vp9"
		profile.PreferredAudioCodec = "opus"
	} else {
		// Default to MP4/H.264 for maximum compatibility
		profile.PreferredContainer = "mp4"
		profile.PreferredVideoCodec = "h264"
		profile.PreferredAudioCodec = "aac"
	}

	// Update device name if not set
	if profile.Name == "Unknown Device" || profile.Name == "" {
		profile.Name = de.detectBrowserName(profile.UserAgent)
	}

	// Add FLAC container support if FLAC codec is supported
	if caps.FLAC && !de.containsString(profile.SupportedContainers, "flac") {
		profile.SupportedContainers = append(profile.SupportedContainers, "flac")
	}

	de.logger.Debug("Enhanced device profile with device capabilities",
		"device", profile.Name,
		"videoCodecs", profile.SupportedVideoCodecs,
		"containers", profile.SupportedContainers,
		"maxResolution", profile.MaxResolution)
}

// detectBrowserName extracts a friendly device/browser name from user agent
func (de *DecisionEngine) detectBrowserName(userAgent string) string {
	ua := strings.ToLower(userAgent)

	switch {
	// Native apps and devices
	case strings.Contains(ua, "viewra/ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS Device"
	case strings.Contains(ua, "viewra/tvos") || strings.Contains(ua, "appletv"):
		return "Apple TV"
	case strings.Contains(ua, "viewra/android"):
		return "Android App"
	case strings.Contains(ua, "nvidia shield"):
		return "Nvidia Shield"
	case strings.Contains(ua, "roku"):
		return "Roku Device"
	case strings.Contains(ua, "smarttv") || strings.Contains(ua, "webos") || strings.Contains(ua, "tizen"):
		return "Smart TV"
	// Web browsers
	case strings.Contains(ua, "chrome"):
		return "Chrome Browser"
	case strings.Contains(ua, "firefox"):
		return "Firefox Browser"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		return "Safari Browser"
	case strings.Contains(ua, "edge"):
		return "Edge Browser"
	case strings.Contains(ua, "opera"):
		return "Opera Browser"
	default:
		return "Web Browser"
	}
}

// isAudioOnlyFile checks if a media file contains only audio streams
func (de *DecisionEngine) isAudioOnlyFile(mediaInfo *types.MediaInfo) bool {
	return playbackutils.IsAudioOnlyFile(mediaInfo)
}

// shouldTranscodeAudio determines if audio should be transcoded for better compatibility
func (de *DecisionEngine) shouldTranscodeAudio(mediaInfo *types.MediaInfo, profile *playbacktypes.DeviceProfile) bool {
	if len(mediaInfo.AudioStreams) == 0 {
		return false
	}

	// Get the primary audio codec
	audioCodec := strings.ToLower(mediaInfo.AudioStreams[0].Codec)

	// Some formats are better transcoded for compatibility
	switch audioCodec {
	case "flac", "alac", "dts", "dts-hd", "truehd":
		// These lossless/high-end formats might benefit from transcoding for streaming
		return true
	case "ac3", "eac3":
		// Dolby formats might need transcoding for some devices
		return !de.hasString(profile.SupportedAudioCodecs, audioCodec)
	}

	return false
}

// containsString checks if a slice contains a string
func (de *DecisionEngine) containsString(slice []string, str string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, str) {
			return true
		}
	}
	return false
}

// hasString checks if a slice contains a string (case-insensitive)
func (de *DecisionEngine) hasString(slice []string, str string) bool {
	return de.containsString(slice, str)
}
