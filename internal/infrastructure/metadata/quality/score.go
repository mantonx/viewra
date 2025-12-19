// Package quality provides heuristic quality scoring for media files.
package quality

import (
	"strings"
)

// MediaParams contains the parameters needed to calculate a quality score.
type MediaParams struct {
	// Video dimensions
	Width  int
	Height int

	// Codecs
	VideoCodec string
	AudioCodec string

	// Bitrate in bits per second
	Bitrate int64

	// HDR format (empty if SDR)
	HDRFormat string

	// Audio channels (for surround detection)
	AudioChannels int
}

// Calculate computes a quality score from 0-100 based on technical characteristics.
// Higher scores indicate better quality.
func Calculate(p MediaParams) int {
	score := 0

	// Resolution score (0-40 points)
	score += resolutionScore(p.Width, p.Height)

	// Codec efficiency score (0-20 points)
	score += codecScore(p.VideoCodec)

	// Bitrate relative to resolution (0-20 points)
	score += bitrateScore(p.Width, p.Height, p.Bitrate)

	// HDR bonus (0-10 points)
	score += hdrScore(p.HDRFormat)

	// Audio quality (0-10 points)
	score += audioScore(p.AudioCodec, p.AudioChannels)

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// resolutionScore returns 0-40 points based on video resolution.
func resolutionScore(width, height int) int {
	pixels := width * height

	switch {
	case pixels >= 3840*2160: // 4K UHD
		return 40
	case pixels >= 2560*1440: // 1440p
		return 35
	case pixels >= 1920*1080: // 1080p
		return 30
	case pixels >= 1280*720: // 720p
		return 20
	case pixels >= 854*480: // 480p
		return 10
	default: // SD or lower
		return 5
	}
}

// codecScore returns 0-20 points based on video codec efficiency.
func codecScore(codec string) int {
	codec = strings.ToLower(codec)

	switch {
	case strings.Contains(codec, "av1"):
		return 20 // Most efficient modern codec
	case strings.Contains(codec, "hevc") || strings.Contains(codec, "h265") || strings.Contains(codec, "x265"):
		return 18 // Excellent efficiency
	case strings.Contains(codec, "vp9"):
		return 16 // Good efficiency
	case strings.Contains(codec, "h264") || strings.Contains(codec, "avc") || strings.Contains(codec, "x264"):
		return 12 // Standard, widely compatible
	case strings.Contains(codec, "vp8"):
		return 10
	case strings.Contains(codec, "mpeg4") || strings.Contains(codec, "divx") || strings.Contains(codec, "xvid"):
		return 6 // Legacy codec
	case strings.Contains(codec, "mpeg2"):
		return 4 // DVD-era codec
	case strings.Contains(codec, "mpeg1"):
		return 2 // Very old
	default:
		return 8 // Unknown codec, assume moderate
	}
}

// bitrateScore returns 0-20 points based on bitrate relative to resolution.
// Higher bitrate for a given resolution means better quality.
func bitrateScore(width, height int, bitrate int64) int {
	if bitrate == 0 || width == 0 || height == 0 {
		return 10 // Unknown, assume average
	}

	pixels := width * height

	// Calculate bits per pixel
	// Good quality is typically 0.1-0.2 bpp for H264, lower for HEVC
	bpp := float64(bitrate) / float64(pixels) / 24 // Assume 24fps

	switch {
	case bpp >= 0.15:
		return 20 // Excellent bitrate
	case bpp >= 0.10:
		return 16 // Good bitrate
	case bpp >= 0.07:
		return 12 // Acceptable
	case bpp >= 0.04:
		return 8 // Low but watchable
	default:
		return 4 // Very low quality
	}
}

// hdrScore returns 0-10 points for HDR content.
func hdrScore(hdrFormat string) int {
	if hdrFormat == "" {
		return 0 // SDR, no bonus
	}

	hdrFormat = strings.ToLower(hdrFormat)

	switch {
	case strings.Contains(hdrFormat, "dolby vision") || strings.Contains(hdrFormat, "dovi"):
		return 10 // Best HDR format
	case strings.Contains(hdrFormat, "hdr10+"):
		return 9 // Dynamic HDR
	case strings.Contains(hdrFormat, "hdr10"):
		return 7 // Static HDR
	case strings.Contains(hdrFormat, "hlg"):
		return 6 // Hybrid Log-Gamma
	default:
		return 5 // Unknown HDR format
	}
}

// audioScore returns 0-10 points based on audio quality.
func audioScore(codec string, channels int) int {
	score := 0
	codec = strings.ToLower(codec)

	// Codec quality (0-5 points)
	switch {
	case strings.Contains(codec, "truehd") || strings.Contains(codec, "atmos"):
		score += 5 // Lossless with object audio
	case strings.Contains(codec, "dts-hd") || strings.Contains(codec, "dtshd"):
		score += 5 // Lossless
	case strings.Contains(codec, "flac") || strings.Contains(codec, "pcm"):
		score += 5 // Lossless
	case strings.Contains(codec, "dts"):
		score += 4 // High quality lossy
	case strings.Contains(codec, "ac3") || strings.Contains(codec, "eac3") || strings.Contains(codec, "dolby"):
		score += 4 // High quality lossy
	case strings.Contains(codec, "aac"):
		score += 3 // Good lossy
	case strings.Contains(codec, "mp3"):
		score += 2 // Standard lossy
	default:
		score += 2 // Unknown
	}

	// Channel count (0-5 points)
	switch {
	case channels >= 8: // 7.1 or Atmos
		score += 5
	case channels >= 6: // 5.1
		score += 4
	case channels >= 2: // Stereo
		score += 2
	case channels == 1: // Mono
		score += 1
	default:
		score += 2 // Unknown, assume stereo
	}

	return score
}
