package playbackmodule

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/pkg/plugins"
)

// PlaybackPlannerImpl implements the PlaybackPlanner interface
type PlaybackPlannerImpl struct{}

// NewPlaybackPlanner creates a new playback planner
func NewPlaybackPlanner() PlaybackPlanner {
	return &PlaybackPlannerImpl{}
}

// DecidePlayback determines whether to direct play or transcode based on media and device capabilities
func (p *PlaybackPlannerImpl) DecidePlayback(mediaPath string, deviceProfile *DeviceProfile) (*PlaybackDecision, error) {
	// Analyze media file
	mediaInfo, err := p.analyzeMedia(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze media: %w", err)
	}

	// Check if direct play is possible
	if p.canDirectPlay(mediaInfo, deviceProfile) {
		return &PlaybackDecision{
			ShouldTranscode: false,
			DirectPlayURL:   mediaPath,
			Reason:          "Media is compatible with client capabilities",
		}, nil
	}

	// Determine transcoding parameters
	transcodeParams, reason := p.determineTranscodeParams(mediaPath, mediaInfo, deviceProfile)

	return &PlaybackDecision{
		ShouldTranscode: true,
		TranscodeParams: transcodeParams,
		Reason:          reason,
	}, nil
}

// canDirectPlay checks if the media can be played directly without transcoding
func (p *PlaybackPlannerImpl) canDirectPlay(media *MediaInfo, profile *DeviceProfile) bool {
	// Check container format
	if !p.isContainerSupported(media.Container, profile) {
		return false
	}

	// Check video codec
	if !p.isCodecSupported(media.VideoCodec, profile.SupportedCodecs) {
		return false
	}

	// Check bitrate limits
	if profile.MaxBitrate > 0 && media.Bitrate > int64(profile.MaxBitrate) {
		return false
	}

	// Check resolution limits
	if !p.isResolutionSupported(media.Resolution, profile.MaxResolution) {
		return false
	}

	// Check HDR support
	if media.HasHDR && !profile.SupportsHDR {
		return false
	}

	return true
}

// isContainerSupported checks if the container format is supported
func (p *PlaybackPlannerImpl) isContainerSupported(container string, profile *DeviceProfile) bool {
	// Web browsers typically don't support MKV directly
	if container == "mkv" && p.isWebBrowser(profile.UserAgent) {
		return false
	}

	// Most modern clients support MP4
	if container == "mp4" {
		return true
	}

	// WebM is supported by most web browsers
	if container == "webm" && p.isWebBrowser(profile.UserAgent) {
		return true
	}

	return false
}

// isCodecSupported checks if the codec is in the supported list
func (p *PlaybackPlannerImpl) isCodecSupported(codec string, supportedCodecs []string) bool {
	for _, supported := range supportedCodecs {
		if strings.EqualFold(codec, supported) {
			return true
		}
	}
	return false
}

// isResolutionSupported checks if the resolution is within limits
func (p *PlaybackPlannerImpl) isResolutionSupported(mediaRes, maxRes string) bool {
	if maxRes == "" {
		return true // No limit specified
	}

	mediaHeight := p.getResolutionHeight(mediaRes)
	maxHeight := p.getResolutionHeight(maxRes)

	return mediaHeight <= maxHeight
}

// getResolutionHeight extracts height from resolution string
func (p *PlaybackPlannerImpl) getResolutionHeight(resolution string) int {
	switch strings.ToLower(resolution) {
	case "480p":
		return 480
	case "720p":
		return 720
	case "1080p":
		return 1080
	case "1440p":
		return 1440
	case "2160p", "4k":
		return 2160
	default:
		return 1080 // Default assumption
	}
}

// isWebBrowser checks if the user agent indicates a web browser
func (p *PlaybackPlannerImpl) isWebBrowser(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	browsers := []string{"chrome", "firefox", "safari", "edge", "opera"}

	for _, browser := range browsers {
		if strings.Contains(userAgent, browser) {
			return true
		}
	}

	return false
}

// determineTranscodeParams determines the optimal transcoding parameters
func (p *PlaybackPlannerImpl) determineTranscodeParams(mediaPath string, media *MediaInfo, profile *DeviceProfile) (*plugins.TranscodeRequest, string) {
	var reasons []string

	// Determine target codec
	targetCodec := p.selectTargetCodec(media.VideoCodec, profile)
	if targetCodec != media.VideoCodec {
		reasons = append(reasons, fmt.Sprintf("codec change: %s -> %s", media.VideoCodec, targetCodec))
	}

	// Determine target resolution
	targetResolution := p.selectTargetResolution(media.Resolution, profile.MaxResolution)
	var resolution *plugins.VideoResolution
	if targetResolution != media.Resolution {
		reasons = append(reasons, fmt.Sprintf("resolution change: %s -> %s", media.Resolution, targetResolution))
		// Convert resolution string to VideoResolution
		height := p.getResolutionHeight(targetResolution)
		width := int(float64(height) * 16.0 / 9.0) // Assume 16:9 aspect ratio
		resolution = &plugins.VideoResolution{
			Width:  width,
			Height: height,
		}
	}

	// Determine target bitrate
	targetBitrate := p.calculateTargetBitrate(targetResolution, profile.MaxBitrate)
	if int64(targetBitrate) < media.Bitrate {
		reasons = append(reasons, fmt.Sprintf("bitrate reduction: %d -> %d", media.Bitrate, targetBitrate))
	}

	// Determine container based on client capabilities and content type
	targetContainer := p.selectTargetContainer(media.Container, profile)
	if media.Container != targetContainer {
		reasons = append(reasons, fmt.Sprintf("container change: %s -> %s", media.Container, targetContainer))
	}

	// Determine quality based on bitrate (0-100 scale)
	quality := p.calculateQuality(targetBitrate)

	// Determine speed priority
	speedPriority := plugins.SpeedPriorityBalanced
	if p.isWebBrowser(profile.UserAgent) {
		speedPriority = plugins.SpeedPriorityFastest // Faster encoding for web clients
	}

	reason := "Transcoding required: " + strings.Join(reasons, ", ")

	return &plugins.TranscodeRequest{
		InputPath:     mediaPath,
		OutputPath:    "", // Will be set by the transcoding service
		VideoCodec:    targetCodec,
		AudioCodec:    "aac",
		Container:     targetContainer,
		Quality:       quality,
		SpeedPriority: speedPriority,
		Resolution:    resolution,
		Seek:          0, // No seek by default
		Duration:      0, // Will use full duration
	}, reason
}

// calculateQuality converts bitrate to quality scale (0-100)
func (p *PlaybackPlannerImpl) calculateQuality(bitrate int) int {
	// Map bitrate to quality
	// Higher bitrate = higher quality
	if bitrate >= 25000 {
		return 90 // Very high quality
	} else if bitrate >= 12000 {
		return 80 // High quality
	} else if bitrate >= 6000 {
		return 70 // Good quality
	} else if bitrate >= 3000 {
		return 60 // Medium quality
	} else if bitrate >= 1500 {
		return 50 // Fair quality
	}
	return 40 // Low quality
}

// selectTargetCodec chooses the best codec for the client
func (p *PlaybackPlannerImpl) selectTargetCodec(sourceCodec string, profile *DeviceProfile) string {
	// Prefer H.264 for maximum compatibility
	if p.isCodecSupported("h264", profile.SupportedCodecs) {
		return "h264"
	}

	// Use HEVC if supported and client supports it
	if profile.SupportsHEVC && p.isCodecSupported("hevc", profile.SupportedCodecs) {
		return "hevc"
	}

	// Fall back to VP8/VP9 for web browsers
	if p.isWebBrowser(profile.UserAgent) {
		if p.isCodecSupported("vp9", profile.SupportedCodecs) {
			return "vp9"
		}
		if p.isCodecSupported("vp8", profile.SupportedCodecs) {
			return "vp8"
		}
	}

	// Default to H.264
	return "h264"
}

// selectTargetResolution chooses the appropriate resolution
func (p *PlaybackPlannerImpl) selectTargetResolution(sourceRes, maxRes string) string {
	if maxRes == "" {
		return sourceRes
	}

	sourceHeight := p.getResolutionHeight(sourceRes)
	maxHeight := p.getResolutionHeight(maxRes)

	if sourceHeight <= maxHeight {
		return sourceRes
	}

	// Downscale to maximum supported resolution
	return maxRes
}

// calculateTargetBitrate calculates appropriate bitrate for resolution
func (p *PlaybackPlannerImpl) calculateTargetBitrate(resolution string, maxBitrate int) int {
	// Base bitrates for different resolutions (in kbps)
	baseBitrates := map[string]int{
		"480p":  1500,
		"720p":  3000,
		"1080p": 6000,
		"1440p": 12000,
		"2160p": 25000,
	}

	targetBitrate := baseBitrates[resolution]
	if targetBitrate == 0 {
		targetBitrate = 6000 // Default to 1080p bitrate
	}

	// Apply client bitrate limit if specified
	if maxBitrate > 0 && targetBitrate > maxBitrate {
		targetBitrate = maxBitrate
	}

	return targetBitrate
}

// selectTargetContainer chooses the best container format for the client
func (p *PlaybackPlannerImpl) selectTargetContainer(sourceContainer string, profile *DeviceProfile) string {
	userAgent := strings.ToLower(profile.UserAgent)

	// Determine if adaptive streaming would be beneficial
	// Use adaptive streaming for:
	// 1. High-resolution content (1440p+) that benefits from adaptive bitrates
	// 2. Long-form content where seeking is important
	// 3. Clients with variable network conditions (mobile, remote)
	shouldUseAdaptiveStreaming := p.shouldUseAdaptiveStreaming(profile)

	if !shouldUseAdaptiveStreaming {
		// For simple cases, prefer progressive MP4 for lower latency and simpler setup
		return "mp4"
	}

	// Choose adaptive streaming format based on client capabilities
	// Modern browsers with MSE support can handle DASH
	if strings.Contains(userAgent, "chrome") || strings.Contains(userAgent, "firefox") || strings.Contains(userAgent, "edge") {
		// Prefer DASH for Chromium-based browsers and Firefox
		return "dash"
	}

	// Safari and iOS have better HLS support
	if strings.Contains(userAgent, "safari") || strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "ios") {
		return "hls"
	}

	// Smart TV and streaming devices often prefer HLS
	if strings.Contains(userAgent, "tv") || strings.Contains(userAgent, "roku") || strings.Contains(userAgent, "appletv") {
		return "hls"
	}

	// Default to DASH for unknown modern clients with MSE support
	return "dash"
}

// shouldUseAdaptiveStreaming determines if adaptive streaming (DASH/HLS) would be beneficial
func (p *PlaybackPlannerImpl) shouldUseAdaptiveStreaming(profile *DeviceProfile) bool {
	// Don't use adaptive streaming for very low resolution content
	if profile.MaxResolution != "" {
		maxHeight := p.getResolutionHeight(profile.MaxResolution)
		if maxHeight < 720 {
			return false // Use progressive for SD content
		}
	}

	// Use adaptive streaming for high-resolution content
	if profile.MaxResolution != "" {
		maxHeight := p.getResolutionHeight(profile.MaxResolution)
		if maxHeight >= 1440 {
			return true // Always use adaptive for 1440p+ content
		}
	}

	// Use adaptive streaming for limited bandwidth scenarios
	if profile.MaxBitrate > 0 && profile.MaxBitrate < 5000 {
		return true // Use adaptive for bandwidth-constrained clients
	}

	// Use adaptive streaming for mobile clients (more variable network conditions)
	userAgent := strings.ToLower(profile.UserAgent)
	if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "ios") {
		return true
	}

	// Use adaptive streaming for remote clients (if we had this information)
	// For now, assume all clients could benefit from adaptive streaming in HD+ content
	if profile.MaxResolution == "" || p.getResolutionHeight(profile.MaxResolution) >= 1080 {
		return true
	}

	// Default to adaptive streaming for modern browsers (Chrome detected in user agent)
	if strings.Contains(userAgent, "chrome") || strings.Contains(userAgent, "firefox") {
		return true
	}

	return false
}

// analyzeMedia extracts media information from the file
func (p *PlaybackPlannerImpl) analyzeMedia(mediaPath string) (*MediaInfo, error) {
	// This is a simplified implementation
	// In a real system, you would use FFprobe or similar tool

	ext := strings.ToLower(filepath.Ext(mediaPath))

	info := &MediaInfo{
		Container:    p.getContainerFromExtension(ext),
		VideoCodec:   "h264",  // Default assumption
		AudioCodec:   "aac",   // Default assumption
		Resolution:   "1080p", // Default assumption
		Bitrate:      6000000, // 6 Mbps default
		Duration:     3600,    // 1 hour default
		HasHDR:       false,
		HasSubtitles: false,
	}

	return info, nil
}

// getContainerFromExtension determines container format from file extension
func (p *PlaybackPlannerImpl) getContainerFromExtension(ext string) string {
	switch ext {
	case ".mp4", ".m4v":
		return "mp4"
	case ".mkv":
		return "mkv"
	case ".avi":
		return "avi"
	case ".webm":
		return "webm"
	case ".mov":
		return "mov"
	default:
		return "unknown"
	}
}
