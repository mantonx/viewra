package playbackmodule

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// PlaybackPlannerImpl implements the PlaybackPlanner interface
type PlaybackPlannerImpl struct {
	// No dependencies - pure decision logic
}

// NewPlaybackPlanner creates a new playback planner instance
func NewPlaybackPlanner() PlaybackPlanner {
	return &PlaybackPlannerImpl{}
}

// DecidePlayback analyzes media file and device to determine playback strategy
func (p *PlaybackPlannerImpl) DecidePlayback(ctx context.Context, mediaPath string, profile DeviceProfile) (*PlaybackDecision, error) {
	// Get media info (this would typically come from metadata extraction)
	mediaInfo, err := p.analyzeMedia(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze media: %w", err)
	}

	// Check container compatibility
	if p.needsContainerTranscode(mediaInfo, profile) {
		return &PlaybackDecision{
			ShouldTranscode: true,
			TranscodeReq: &TranscodeRequest{
				InputPath:   mediaPath,
				TargetCodec: p.selectBestCodec(profile),
				Resolution:  p.selectBestResolution(mediaInfo, profile),
				Bitrate:     p.calculateOptimalBitrate(mediaInfo, profile),
				DeviceHints: profile,
			},
			Reason: fmt.Sprintf("Container %s not supported, transcoding required", mediaInfo.Container),
		}, nil
	}

	// Check codec compatibility
	if p.needsCodecTranscode(mediaInfo, profile) {
		return &PlaybackDecision{
			ShouldTranscode: true,
			TranscodeReq: &TranscodeRequest{
				InputPath:   mediaPath,
				TargetCodec: p.selectBestCodec(profile),
				Resolution:  p.selectBestResolution(mediaInfo, profile),
				Bitrate:     p.calculateOptimalBitrate(mediaInfo, profile),
				DeviceHints: profile,
			},
			Reason: fmt.Sprintf("Codec %s not supported by device", mediaInfo.VideoCodec),
		}, nil
	}

	// Check bitrate/quality requirements
	if p.needsBitrateTranscode(mediaInfo, profile) {
		return &PlaybackDecision{
			ShouldTranscode: true,
			TranscodeReq: &TranscodeRequest{
				InputPath:   mediaPath,
				TargetCodec: mediaInfo.VideoCodec, // Keep same codec, just lower bitrate
				Resolution:  p.selectBestResolution(mediaInfo, profile),
				Bitrate:     profile.MaxBitrate,
				DeviceHints: profile,
			},
			Reason: fmt.Sprintf("Bitrate %d exceeds device limit %d", mediaInfo.Bitrate, profile.MaxBitrate),
		}, nil
	}

	// Direct playback is fine
	return &PlaybackDecision{
		ShouldTranscode: false,
		DirectPath:      mediaPath,
		Reason:          "Media compatible with device capabilities",
	}, nil
}

// analyzeMedia extracts metadata from media file
func (p *PlaybackPlannerImpl) analyzeMedia(mediaPath string) (*MediaInfo, error) {
	// For now, detect based on file extension
	// In a real implementation, this would use FFprobe or similar
	ext := strings.ToLower(filepath.Ext(mediaPath))
	
	switch ext {
	case ".mkv":
		return &MediaInfo{
			Container:    "matroska",
			VideoCodec:   "h264", // Assumption
			AudioCodec:   "aac",
			Resolution:   "1080p",
			Bitrate:      5000000, // 5 Mbps assumption
			Duration:     3600,    // 1 hour assumption
			HasHDR:       false,
			HasSubtitles: true,
		}, nil
	case ".mp4":
		return &MediaInfo{
			Container:    "mp4",
			VideoCodec:   "h264",
			AudioCodec:   "aac",
			Resolution:   "1080p",
			Bitrate:      3000000, // 3 Mbps assumption
			Duration:     3600,
			HasHDR:       false,
			HasSubtitles: false,
		}, nil
	case ".webm":
		return &MediaInfo{
			Container:    "webm",
			VideoCodec:   "vp9",
			AudioCodec:   "opus",
			Resolution:   "1080p",
			Bitrate:      2000000, // 2 Mbps assumption
			Duration:     3600,
			HasHDR:       false,
			HasSubtitles: false,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// needsContainerTranscode checks if container format is incompatible
func (p *PlaybackPlannerImpl) needsContainerTranscode(media *MediaInfo, profile DeviceProfile) bool {
	// MKV is problematic for web playback
	if media.Container == "matroska" {
		return true
	}
	
	// Most other containers should work
	return false
}

// needsCodecTranscode checks if codec is incompatible with device
func (p *PlaybackPlannerImpl) needsCodecTranscode(media *MediaInfo, profile DeviceProfile) bool {
	// Check if device supports the codec
	for _, codec := range profile.SupportedCodecs {
		if strings.EqualFold(codec, media.VideoCodec) {
			return false
		}
	}
	
	// Special cases for modern codecs
	if media.VideoCodec == "hevc" && !profile.SupportsHEVC {
		return true
	}
	if media.VideoCodec == "av1" && !profile.SupportsAV1 {
		return true
	}
	
	return false
}

// needsBitrateTranscode checks if bitrate exceeds device capabilities
func (p *PlaybackPlannerImpl) needsBitrateTranscode(media *MediaInfo, profile DeviceProfile) bool {
	if profile.MaxBitrate > 0 && int(media.Bitrate) > profile.MaxBitrate {
		return true
	}
	return false
}

// selectBestCodec chooses optimal codec for device
func (p *PlaybackPlannerImpl) selectBestCodec(profile DeviceProfile) string {
	// Prefer modern codecs if supported
	if profile.SupportsAV1 {
		return "av1"
	}
	if profile.SupportsHEVC {
		return "hevc"
	}
	
	// Fall back to H.264 (universal support)
	return "h264"
}

// selectBestResolution chooses optimal resolution for device
func (p *PlaybackPlannerImpl) selectBestResolution(media *MediaInfo, profile DeviceProfile) string {
	maxRes := profile.MaxResolution
	if maxRes == "" {
		maxRes = "1080p" // Default
	}
	
	// Don't upscale
	mediaRes := media.Resolution
	if p.isResolutionLower(mediaRes, maxRes) {
		return mediaRes
	}
	
	return maxRes
}

// calculateOptimalBitrate determines best bitrate for transcoding
func (p *PlaybackPlannerImpl) calculateOptimalBitrate(media *MediaInfo, profile DeviceProfile) int {
	// Start with source bitrate
	bitrate := int(media.Bitrate)
	
	// Cap at device maximum
	if profile.MaxBitrate > 0 && bitrate > profile.MaxBitrate {
		bitrate = profile.MaxBitrate
	}
	
	// Apply reasonable minimums based on resolution
	switch media.Resolution {
	case "2160p":
		if bitrate < 8000000 {
			bitrate = 8000000 // 8 Mbps minimum for 4K
		}
	case "1440p":
		if bitrate < 5000000 {
			bitrate = 5000000 // 5 Mbps minimum for 1440p
		}
	case "1080p":
		if bitrate < 2500000 {
			bitrate = 2500000 // 2.5 Mbps minimum for 1080p
		}
	case "720p":
		if bitrate < 1500000 {
			bitrate = 1500000 // 1.5 Mbps minimum for 720p
		}
	}
	
	return bitrate
}

// isResolutionLower compares two resolution strings
func (p *PlaybackPlannerImpl) isResolutionLower(res1, res2 string) bool {
	resOrder := map[string]int{
		"480p":  1,
		"720p":  2,
		"1080p": 3,
		"1440p": 4,
		"2160p": 5,
	}
	
	return resOrder[res1] < resOrder[res2]
} 