// Package utils provides playback-specific utility functions.
package utils

import (
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/types"
	"github.com/mantonx/viewra/internal/utils"
)

// IsAudioOnlyFile checks if a media file contains only audio streams
func IsAudioOnlyFile(mediaInfo *types.MediaInfo) bool {
	return len(mediaInfo.VideoStreams) == 0 && len(mediaInfo.AudioStreams) > 0
}

// IsVideoFile checks if a media file contains video streams
func IsVideoFile(mediaInfo *types.MediaInfo) bool {
	return len(mediaInfo.VideoStreams) > 0
}

// GetPrimaryVideoCodec returns the codec of the primary video stream
func GetPrimaryVideoCodec(mediaInfo *types.MediaInfo) string {
	if len(mediaInfo.VideoStreams) > 0 {
		return mediaInfo.VideoStreams[0].Codec
	}
	return ""
}

// GetPrimaryAudioCodec returns the codec of the primary audio stream
func GetPrimaryAudioCodec(mediaInfo *types.MediaInfo) string {
	if len(mediaInfo.AudioStreams) > 0 {
		return mediaInfo.AudioStreams[0].Codec
	}
	return ""
}

// FormatResolution converts width and height into a resolution string (e.g., "1080p")
func FormatResolution(width, height int) string {
	// Common resolutions
	switch height {
	case 480:
		return "480p"
	case 720:
		return "720p"
	case 1080:
		return "1080p"
	case 1440:
		return "1440p"
	case 2160:
		return "2160p"
	default:
		return fmt.Sprintf("%dp", height)
	}
}

// ParseResolution parses a resolution string (e.g., "1080p", "4K") into height
func ParseResolution(resolution string) int {
	resolution = strings.ToLower(resolution)

	// Handle special cases
	switch resolution {
	case "4k", "uhd":
		return 2160
	case "2k", "qhd":
		return 1440
	case "fhd", "fullhd":
		return 1080
	case "hd":
		return 720
	case "sd":
		return 480
	}

	// Try to parse numeric format (e.g., "1080p")
	resolution = strings.TrimSuffix(resolution, "p")
	var height int
	fmt.Sscanf(resolution, "%d", &height)
	return height
}

// IsLosslessAudioCodec checks if an audio codec is lossless
func IsLosslessAudioCodec(codec string) bool {
	codec = strings.ToLower(codec)
	losslessCodecs := []string{"flac", "alac", "ape", "wav", "pcm", "dts-hd", "truehd"}

	for _, lossless := range losslessCodecs {
		if strings.Contains(codec, lossless) {
			return true
		}
	}
	return false
}

// GetRecommendedBitrate returns recommended bitrate for a given resolution and codec
func GetRecommendedBitrate(resolution string, codec string) int {
	height := ParseResolution(resolution)
	codec = strings.ToLower(codec)

	// Base bitrates for H.264
	baseBitrates := map[int]int{
		480:  2500000,  // 2.5 Mbps
		720:  5000000,  // 5 Mbps
		1080: 8000000,  // 8 Mbps
		1440: 16000000, // 16 Mbps
		2160: 35000000, // 35 Mbps
	}

	// Get base bitrate
	bitrate, exists := baseBitrates[height]
	if !exists {
		// Estimate based on height
		bitrate = height * height * 3
	}

	// Adjust for codec efficiency
	switch codec {
	case "h265", "hevc":
		bitrate = bitrate * 60 / 100 // H.265 is ~40% more efficient
	case "vp9":
		bitrate = bitrate * 65 / 100 // VP9 is ~35% more efficient
	case "av1":
		bitrate = bitrate * 50 / 100 // AV1 is ~50% more efficient
	}

	return bitrate
}

// ShouldTranscodeForStreaming determines if a file should be transcoded for optimal streaming
func ShouldTranscodeForStreaming(mediaInfo *types.MediaInfo) bool {
	// Check if it's already using the shared utils
	if !utils.IsMediaFile(mediaInfo.Path) {
		return false
	}

	// High bitrate files benefit from transcoding
	if mediaInfo.Bitrate > 50000000 { // 50 Mbps
		return true
	}

	// Lossless audio might need transcoding for streaming
	if IsAudioOnlyFile(mediaInfo) && len(mediaInfo.AudioStreams) > 0 {
		if IsLosslessAudioCodec(mediaInfo.AudioStreams[0].Codec) {
			return true
		}
	}

	// Some containers don't stream well
	container := strings.ToLower(mediaInfo.Container)
	problematicContainers := []string{"avi", "wmv", "flv"}
	for _, prob := range problematicContainers {
		if container == prob {
			return true
		}
	}

	return false
}
