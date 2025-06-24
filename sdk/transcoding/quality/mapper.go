// Package quality provides quality mapping and optimization for video encoding.
// This package translates abstract quality levels (0-100) into codec-specific
// parameters that achieve optimal visual quality while maintaining reasonable
// file sizes. It understands the nuances of different codecs and their quality
// scales.
//
// The quality mapper handles:
// - Mapping quality percentages to CRF/QP values
// - Codec-specific quality optimizations
// - Bitrate recommendations based on quality targets
// - Preset selection for quality/speed trade-offs
// - Resolution-aware quality adjustments
//
// Supported codec mappings:
// - H.264: CRF 0-51 scale with perceptual optimization
// - H.265: CRF 0-51 scale with improved efficiency
// - VP9: CQ 0-63 scale with rate control
// - AV1: CQ 0-63 scale with advanced psychovisual tuning
package quality

import (
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// FFmpegQualityMapper implements QualityMapper for FFmpeg
type FFmpegQualityMapper struct {
	crfRanges map[string]crfRange
}

type crfRange struct {
	min, max int
	optimal  int
}

// NewFFmpegQualityMapper creates a new FFmpeg quality mapper
func NewFFmpegQualityMapper() *FFmpegQualityMapper {
	return &FFmpegQualityMapper{
		crfRanges: map[string]crfRange{
			"h264": {min: 0, max: 51, optimal: 23},
			"h265": {min: 0, max: 51, optimal: 28},
			"vp9":  {min: 0, max: 63, optimal: 31},
			"av1":  {min: 0, max: 63, optimal: 30},
			"vp8":  {min: 0, max: 63, optimal: 31},
		},
	}
}

// MapQuality converts 0-100 quality percentage to FFmpeg CRF
func (m *FFmpegQualityMapper) MapQuality(percent int, codec string) map[string]interface{} {
	// Clamp percentage to valid range
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}

	// Get CRF range for codec
	r, exists := m.crfRanges[codec]
	if !exists {
		// Default to h264 if codec not found
		r = m.crfRanges["h264"]
	}

	// Map percentage to CRF (inverted scale)
	// 0% quality = worst (highest CRF)
	// 100% quality = best (lowest CRF)
	crf := r.max - int(float64(percent)/100.0*float64(r.max-r.min))

	return map[string]interface{}{
		"crf": crf,
	}
}

// GetSpeedPreset maps generic speed priority to FFmpeg preset
func (m *FFmpegQualityMapper) GetSpeedPreset(priority types.SpeedPriority) string {
	switch priority {
	case types.SpeedPriorityFastest:
		return "ultrafast"
	case types.SpeedPriorityBalanced:
		return "fast"
	case types.SpeedPriorityQuality:
		return "slow"
	default:
		return "fast"
	}
}

// GetDefaultQuality returns the default quality percentage for a codec
func (m *FFmpegQualityMapper) GetDefaultQuality(codec string) int {
	// Calculate percentage for optimal CRF
	// optimal CRF should map to a reasonable quality percentage
	switch codec {
	case "h264":
		return 55 // CRF 23 ≈ 55% quality
	case "h265":
		return 45 // CRF 28 ≈ 45% quality (HEVC is more efficient)
	case "vp9":
		return 50 // CRF 31 ≈ 50% quality
	case "av1":
		return 52 // CRF 30 ≈ 52% quality
	default:
		return 50
	}
}
