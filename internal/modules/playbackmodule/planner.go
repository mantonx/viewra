package playbackmodule

import (
	"fmt"
	"path/filepath"
	"strings"

	plugins "github.com/mantonx/viewra/sdk"
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
			StreamURL:       mediaPath, // For frontend compatibility
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
	var resolution *plugins.Resolution
	if targetResolution != media.Resolution {
		reasons = append(reasons, fmt.Sprintf("resolution change: %s -> %s", media.Resolution, targetResolution))
		// Convert resolution string to VideoResolution
		height := p.getResolutionHeight(targetResolution)
		width := int(float64(height) * 16.0 / 9.0) // Assume 16:9 aspect ratio
		resolution = &plugins.Resolution{
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

	// ENHANCED: Smart ABR enablement based on device profile
	enableABR := p.shouldEnableABR(targetContainer, profile, media)
	if enableABR {
		reasons = append(reasons, "enabling adaptive bitrate streaming for optimal experience")
	}

	// Determine speed priority based on device capabilities
	speedPriority := p.determineSpeedPriority(profile)

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
		// Duration field removed - not in TranscodeRequest
		EnableABR:     enableABR,
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
	// Use device-specific container selection for optimal playback
	userAgent := strings.ToLower(profile.UserAgent)
	
	// iOS devices and Safari prefer HLS (native support)
	if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || 
	   (strings.Contains(userAgent, "safari") && !strings.Contains(userAgent, "chrome")) {
		return "hls"
	}
	
	// All other devices (Android, Desktop) use DASH for better performance
	// DASH provides:
	// - Better seeking performance with static MPD
	// - More efficient segment structure
	// - Superior adaptive streaming algorithms
	return "dash"
}

// shouldEnableABR determines if adaptive bitrate streaming should be enabled
// based on device capabilities and content characteristics
func (p *PlaybackPlannerImpl) shouldEnableABR(container string, profile *DeviceProfile, media *MediaInfo) bool {
	// Only enable ABR for adaptive streaming containers
	if container != "dash" && container != "hls" {
		return false
	}
	
	// Check if device supports ABR based on performance level
	userAgent := strings.ToLower(profile.UserAgent)
	
	// Always enable ABR for desktop and high-performance devices
	if strings.Contains(userAgent, "chrome") || strings.Contains(userAgent, "firefox") || 
	   strings.Contains(userAgent, "edge") || strings.Contains(userAgent, "safari") {
		return true
	}
	
	// Enable ABR for mobile devices with sufficient bandwidth
	if profile.MaxBitrate >= 2000 { // 2 Mbps minimum for effective ABR
		return true
	}
	
	// Enable ABR for long content (>10 minutes) where network conditions may vary
	if media.Duration > 600 { // 10 minutes
		return true
	}
	
	// Enable ABR for high bitrate content that benefits from adaptation
	if media.Bitrate > 5000000 { // 5 Mbps
		return true
	}
	
	// Default to single bitrate for simple scenarios
	return false
}

// determineSpeedPriority selects encoding speed based on device capabilities
func (p *PlaybackPlannerImpl) determineSpeedPriority(profile *DeviceProfile) plugins.SpeedPriority {
	userAgent := strings.ToLower(profile.UserAgent)
	
	// Mobile devices prefer faster encoding for battery conservation
	if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") ||
	   strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") {
		return plugins.SpeedPriorityFastest
	}
	
	// Desktop browsers prefer balanced encoding
	if p.isWebBrowser(profile.UserAgent) {
		return plugins.SpeedPriorityBalanced
	}
	
	// High bitrate connections can afford quality-focused encoding
	if profile.MaxBitrate >= 10000 { // 10 Mbps+
		return plugins.SpeedPriorityQuality
	}
	
	// Default to balanced
	return plugins.SpeedPriorityBalanced
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
