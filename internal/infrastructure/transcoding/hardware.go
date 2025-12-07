package transcoding

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/ffmpeg"
)

// VideoCodec represents supported video codecs for transcoding
// Re-exported from ffmpeg subpackage for backward compatibility
type VideoCodec = ffmpeg.VideoCodec

const (
	CodecH264 = ffmpeg.CodecH264
	CodecH265 = ffmpeg.CodecH265
	CodecVP9  = ffmpeg.CodecVP9
	CodecAV1  = ffmpeg.CodecAV1
)

// GetVideoCodecAndPreset returns the appropriate video codec and preset for a hardware acceleration type.
// Re-exported from ffmpeg subpackage for backward compatibility
func GetVideoCodecAndPreset(hwAccel HardwareAccel) (codec string, preset string) {
	return ffmpeg.GetVideoCodecAndPreset(ffmpeg.HardwareAccel(hwAccel))
}

// GetVideoCodecAndPresetForCodec returns the FFmpeg encoder name and preset for a given codec and hardware acceleration.
// Re-exported from ffmpeg subpackage for backward compatibility
func GetVideoCodecAndPresetForCodec(hwAccel HardwareAccel, videoCodec VideoCodec) (encoder string, preset string) {
	return ffmpeg.GetVideoCodecAndPresetForCodec(ffmpeg.HardwareAccel(hwAccel), videoCodec)
}

// IsCodecSupported checks if a codec is supported for the given hardware acceleration
// Re-exported from ffmpeg subpackage for backward compatibility
func IsCodecSupported(hwAccel HardwareAccel, videoCodec VideoCodec) bool {
	return ffmpeg.IsCodecSupported(ffmpeg.HardwareAccel(hwAccel), videoCodec)
}

// GetBestCodecForHardware returns the best codec for bandwidth savings given hardware support
// Re-exported from ffmpeg subpackage for backward compatibility
func GetBestCodecForHardware(hwAccel HardwareAccel, clientCodecs []string) VideoCodec {
	return ffmpeg.GetBestCodecForHardware(ffmpeg.HardwareAccel(hwAccel), clientCodecs)
}

// GetHardwareAccelArgs returns FFmpeg arguments for hardware acceleration.
// Re-exported from ffmpeg subpackage for backward compatibility
func GetHardwareAccelArgs(hwAccel HardwareAccel) []string {
	return ffmpeg.GetHardwareAccelArgs(ffmpeg.HardwareAccel(hwAccel))
}

// GetHardwareAccelArgsWithDevice returns FFmpeg arguments for hardware acceleration with a specific device.
// Re-exported from ffmpeg subpackage for backward compatibility
func GetHardwareAccelArgsWithDevice(hwAccel HardwareAccel, device string) []string {
	return ffmpeg.GetHardwareAccelArgsWithDevice(ffmpeg.HardwareAccel(hwAccel), device)
}

// CheckFFmpegEncoder verifies that FFmpeg has support for a specific encoder.
// Re-exported from ffmpeg subpackage for backward compatibility
func CheckFFmpegEncoder(encoderName string) bool {
	return ffmpeg.CheckFFmpegEncoder(encoderName)
}

// CheckFFmpegFilter verifies that FFmpeg has support for a specific filter.
// Re-exported from ffmpeg subpackage for backward compatibility
func CheckFFmpegFilter(filterName string) bool {
	return ffmpeg.CheckFFmpegFilter(filterName)
}

// HardwareFallbackManager wraps ffmpeg.HardwareFallbackManager to handle type conversions
type HardwareFallbackManager struct {
	inner *ffmpeg.HardwareFallbackManager
}

// NewHardwareFallbackManager creates a new fallback manager with default retry logic.
func NewHardwareFallbackManager(config *TranscodeConfig, logger *slog.Logger) *HardwareFallbackManager {
	return &HardwareFallbackManager{
		inner: ffmpeg.NewHardwareFallbackManager(convertToFFmpegConfig(config), logger),
	}
}

// GetCurrentAccel returns the currently active hardware acceleration method.
func (m *HardwareFallbackManager) GetCurrentAccel() HardwareAccel {
	return HardwareAccel(m.inner.GetCurrentAccel())
}

// RecordFailure records a hardware encoding failure and potentially triggers fallback.
func (m *HardwareFallbackManager) RecordFailure(hwAccel HardwareAccel, err error) bool {
	return m.inner.RecordFailure(ffmpeg.HardwareAccel(hwAccel), err)
}

// VerifyHardwareAvailability proactively tests if hardware acceleration is working.
func (m *HardwareFallbackManager) VerifyHardwareAvailability(hwAccel HardwareAccel) error {
	return m.inner.VerifyHardwareAvailability(ffmpeg.HardwareAccel(hwAccel))
}

// ResetFailureCount resets the failure count for successful encodes.
func (m *HardwareFallbackManager) ResetFailureCount(hwAccel HardwareAccel) {
	m.inner.ResetFailureCount(ffmpeg.HardwareAccel(hwAccel))
}

// PlaylistMetadata contains custom metadata from an HLS playlist.
// Re-exported from ffmpeg subpackage for backward compatibility
type PlaylistMetadata = ffmpeg.PlaylistMetadata

// ParsePlaylistMetadata reads custom extension tags from an M3U8 playlist.
// Re-exported from ffmpeg subpackage for backward compatibility
func ParsePlaylistMetadata(playlistContent string) PlaylistMetadata {
	return ffmpeg.ParsePlaylistMetadata(playlistContent)
}

// NewFFmpegArgsBuilder creates a new FFmpeg arguments builder.
// Deprecated: Use ffmpeg.NewFFmpegArgsBuilder directly
func NewFFmpegArgsBuilder(opts TranscodeOptions) interface{} {
	// This is deprecated and should not be used - convert through ffmpeg package
	panic("NewFFmpegArgsBuilder is deprecated - use ffmpeg.NewFFmpegArgsBuilder with converted options")
}
