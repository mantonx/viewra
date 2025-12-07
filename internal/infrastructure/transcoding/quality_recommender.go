package transcoding

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// ClientCapabilities is re-exported from the profile subpackage for backward compatibility.
type ClientCapabilities = profile.ClientCapabilities

// QualityRecommendation is re-exported from the profile subpackage for backward compatibility.
type QualityRecommendation = profile.QualityRecommendation

// QualityRecommender wraps the profile.QualityRecommender and adds codec-aware methods.
// The base quality recommendation logic is in the profile subpackage; this wrapper
// adds methods that require hardware acceleration types from this package.
type QualityRecommender struct {
	inner *profile.QualityRecommender
	logger *slog.Logger
}

// NewQualityRecommender creates a new quality recommender.
func NewQualityRecommender(logger *slog.Logger) *QualityRecommender {
	return &QualityRecommender{
		inner:  profile.NewQualityRecommender(logger),
		logger: logger,
	}
}

// RecommendQuality recommends the best quality profile for given client capabilities.
// This provides an initial quality hint to HLS.js, which will then adapt based on
// actual network performance.
func (qr *QualityRecommender) RecommendQuality(caps ClientCapabilities) *QualityRecommendation {
	return qr.inner.RecommendQuality(caps)
}

// RecommendMultipleQualities returns top N recommended quality profiles.
func (qr *QualityRecommender) RecommendMultipleQualities(caps ClientCapabilities, count int) []*QualityRecommendation {
	return qr.inner.RecommendMultipleQualities(caps, count)
}

// GetAdaptiveLadder returns a set of quality profiles for adaptive streaming (ABR ladder).
func (qr *QualityRecommender) GetAdaptiveLadder(caps ClientCapabilities) []*AdaptiveProfile {
	return qr.inner.GetAdaptiveLadder(caps)
}

// RecommendCodec recommends the best codec for the given client capabilities and hardware.
// Prioritizes: AV1 > H.265 > VP9 > H.264 based on compression efficiency.
// Falls back to H.264 if no modern codec is supported.
func (qr *QualityRecommender) RecommendCodec(caps ClientCapabilities, hwAccel HardwareAccel) VideoCodec {
	// Build a set of client-supported codecs for fast lookup
	supported := make(map[string]bool)
	for _, c := range caps.SupportedCodecs {
		supported[c] = true
	}

	qr.logger.Debug("recommending codec",
		"supported_codecs", caps.SupportedCodecs,
		"hw_accel", hwAccel,
	)

	// Check codecs in order of compression efficiency
	// AV1 > H.265/VP9 > H.264

	// Try AV1 (best compression, 30% better than H.265)
	if supported["av1"] && IsCodecSupported(hwAccel, CodecAV1) {
		qr.logger.Info("recommending codec", "codec", "av1", "reason", "best compression")
		return CodecAV1
	}

	// Try H.265 (25% better compression than H.264, good browser support)
	if supported["h265"] && IsCodecSupported(hwAccel, CodecH265) {
		qr.logger.Info("recommending codec", "codec", "h265", "reason", "good compression")
		return CodecH265
	}

	// Try VP9 (similar to H.265, Chrome/Firefox preferred)
	if supported["vp9"] && IsCodecSupported(hwAccel, CodecVP9) {
		qr.logger.Info("recommending codec", "codec", "vp9", "reason", "royalty-free")
		return CodecVP9
	}

	// Fall back to H.264 (universal compatibility)
	qr.logger.Info("recommending codec", "codec", "h264", "reason", "universal fallback")
	return CodecH264
}

// GetBestCodecForProfile returns the best codec for a specific quality profile.
// It considers both client support and the profile's preferred/fallback codecs.
func (qr *QualityRecommender) GetBestCodecForProfile(profile *AdaptiveProfile, caps ClientCapabilities, hwAccel HardwareAccel) VideoCodec {
	// Build supported codec set
	supported := make(map[string]bool)
	for _, c := range caps.SupportedCodecs {
		supported[c] = true
	}

	// Check if preferred codec is supported
	if supported[profile.PreferredCodec] && IsCodecSupported(hwAccel, VideoCodec(profile.PreferredCodec)) {
		return VideoCodec(profile.PreferredCodec)
	}

	// Check fallback codecs in order
	for _, fallback := range profile.FallbackCodecs {
		if supported[fallback] && IsCodecSupported(hwAccel, VideoCodec(fallback)) {
			return VideoCodec(fallback)
		}
	}

	// Last resort: H.264
	return CodecH264
}
