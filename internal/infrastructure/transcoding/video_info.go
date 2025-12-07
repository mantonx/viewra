package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/probe"
)

// AudioTrack is re-exported from the probe subpackage for backward compatibility.
type AudioTrack = probe.AudioTrack

// VideoInfo is re-exported from the probe subpackage for backward compatibility.
type VideoInfo = probe.VideoInfo

// GetVideoInfo extracts video metadata using ffprobe.
// Results are cached to avoid repeated ffprobe calls for the same file.
// Re-exported from probe subpackage for backward compatibility.
func GetVideoInfo(inputPath string) (*VideoInfo, error) {
	return probe.GetVideoInfo(inputPath)
}

// SelectBestAudioTrack chooses the most suitable audio track from available options.
// Re-exported from probe subpackage for backward compatibility.
func SelectBestAudioTrack(tracks []AudioTrack) *AudioTrack {
	return probe.SelectBestAudioTrack(tracks)
}

// DetectBitDepthFromPixelFormat determines bit depth from pixel format string.
// Re-exported from probe subpackage for backward compatibility.
func DetectBitDepthFromPixelFormat(pixFmt string) int {
	return probe.DetectBitDepthFromPixelFormat(pixFmt)
}

// IsHDRContent determines if video content is HDR based on color metadata.
// Re-exported from probe subpackage for backward compatibility.
func IsHDRContent(colorTransfer, colorPrimaries string) bool {
	return probe.IsHDRContent(colorTransfer, colorPrimaries)
}

// VideoInfoCache provides access to the global video info cache.
// Use this for cache management operations like clearing or checking size.
var VideoInfoCache = probe.GlobalCache
