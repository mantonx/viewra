package transcoding

import (
	"log/slog"
)

// ClientCapabilities represents the detected capabilities of a client device.
type ClientCapabilities struct {
	// Network
	NetworkSpeedMbps float64
	ConnectionType   string // "wifi", "4g", "5g", "ethernet", etc.
	IsMetered        bool   // True if on a metered connection (cellular)

	// Device
	DeviceType   string // "mobile", "tablet", "desktop", "tv"
	ScreenWidth  int
	ScreenHeight int
	PixelRatio   float64

	// Performance (optional, not used for quality selection)
	CPUCores     int
	MemoryGB     float64
	LowPowerMode bool
	BatteryLevel float64
	IsCharging   bool

	// Media capabilities
	SupportedCodecs      []string
	HardwareAcceleration bool
	MaxDecodingProfile   string // e.g., "1080p-60fps", "4k-30fps"
}

// QualityRecommendation represents a recommended quality profile with score.
type QualityRecommendation struct {
	Profile *AdaptiveProfile
	Score   float64
	Reason  string
}

// QualityRecommender recommends optimal quality profiles based on client capabilities.
type QualityRecommender struct {
	logger *slog.Logger
}

// NewQualityRecommender creates a new quality recommender.
func NewQualityRecommender(logger *slog.Logger) *QualityRecommender {
	return &QualityRecommender{
		logger: logger,
	}
}

// RecommendQuality recommends the best quality profile for given client capabilities.
// This provides an initial quality hint to HLS.js, which will then adapt based on
// actual network performance. Keep it simple - HLS.js does the heavy lifting.
func (qr *QualityRecommender) RecommendQuality(caps ClientCapabilities) *QualityRecommendation {
	networkMbps := caps.NetworkSpeedMbps
	screenHeight := caps.ScreenHeight

	// If network speed is unknown/failed, assume a decent connection
	// HLS.js will measure actual throughput and adapt accordingly
	if networkMbps <= 0 {
		networkMbps = 50.0
	}

	qr.logger.Info("recommending quality",
		"network_mbps", networkMbps,
		"screen_height", screenHeight,
		"device", caps.DeviceType,
	)

	// Simple bandwidth + screen resolution selection
	// HLS.js will adapt from this starting point based on real network performance
	var profileID string
	var reason string

	switch {
	// 4K selection - need fast network AND 4K screen
	case networkMbps >= 60 && screenHeight >= 2160:
		profileID = Quality4k60m
		reason = "4K screen with excellent network"
	case networkMbps >= 40 && screenHeight >= 2160:
		profileID = Quality4k40m
		reason = "4K screen with good network"
	case networkMbps >= 25 && screenHeight >= 2160:
		profileID = Quality4k25m
		reason = "4K screen with moderate network"
	case networkMbps >= 15 && screenHeight >= 2160:
		profileID = Quality4k15m
		reason = "4K screen with acceptable network"

	// 1080p selection - most common case
	case networkMbps >= 40 && screenHeight >= 1080:
		profileID = Quality1080p40m
		reason = "1080p screen with excellent network"
	case networkMbps >= 20 && screenHeight >= 1080:
		profileID = Quality1080p20m
		reason = "1080p screen with good network"
	case networkMbps >= 10 && screenHeight >= 1080:
		profileID = Quality1080p10m
		reason = "1080p screen with moderate network"
	case networkMbps >= 4 && screenHeight >= 1080:
		profileID = Quality1080p4m
		reason = "1080p screen with limited network"

	// 720p selection
	case networkMbps >= 4 && screenHeight >= 720:
		profileID = Quality720p4m
		reason = "720p screen with moderate network"
	case networkMbps >= 2 && screenHeight >= 720:
		profileID = Quality720p2m
		reason = "720p screen with limited network"

	// SD fallbacks
	case networkMbps >= 1:
		profileID = Quality480p
		reason = "SD fallback for poor network"
	default:
		profileID = Quality360p
		reason = "lowest quality for very poor connection"
	}

	// Apply metered connection penalty - drop down one tier
	if caps.IsMetered && networkMbps > 5 {
		reason += " (reduced for metered connection)"
		// Simple downgrade logic
		switch profileID {
		case Quality4k60m, Quality4k40m:
			profileID = Quality4k25m
		case Quality4k25m:
			profileID = Quality4k15m
		case Quality4k15m:
			profileID = Quality1080p20m
		case Quality1080p60m, Quality1080p40m:
			profileID = Quality1080p20m
		case Quality1080p20m:
			profileID = Quality1080p10m
		case Quality1080p10m:
			profileID = Quality1080p4m
		case Quality1080p4m:
			profileID = Quality720p4m
		case Quality720p4m:
			profileID = Quality720p2m
		}
	}

	profile, err := GetAdaptiveProfile(profileID)
	if err != nil {
		// Fallback to safe default
		profile, _ = GetAdaptiveProfile(Quality480p)
		reason = "fallback to safe default"
	}

	qr.logger.Info("quality recommendation",
		"selected_profile", profileID,
		"height", profile.Height,
		"reason", reason,
	)

	return &QualityRecommendation{
		Profile: profile,
		Score:   1.0,
		Reason:  reason,
	}
}

// RecommendMultipleQualities returns top N recommended quality profiles.
// For adaptive streaming, this is simplified - just return profiles that fit
// within the network and screen constraints.
func (qr *QualityRecommender) RecommendMultipleQualities(caps ClientCapabilities, count int) []*QualityRecommendation {
	// For now, just return the single best recommendation
	// HLS.js handles the adaptive ladder itself
	recommendation := qr.RecommendQuality(caps)
	return []*QualityRecommendation{recommendation}
}

// GetAdaptiveLadder returns a set of quality profiles for adaptive streaming (ABR ladder).
// Returns a simplified ladder - HLS.js will select from available variants in the master playlist.
func (qr *QualityRecommender) GetAdaptiveLadder(caps ClientCapabilities) []*AdaptiveProfile {
	networkMbps := caps.NetworkSpeedMbps
	if networkMbps <= 0 {
		networkMbps = 50.0
	}

	screenHeight := caps.ScreenHeight

	// Build a simple ladder with key quality levels
	ladder := make([]*AdaptiveProfile, 0, 6)

	// Always include lowest quality for poor conditions
	if profile, err := GetAdaptiveProfile(Quality360p); err == nil {
		ladder = append(ladder, profile)
	}

	// Add intermediate qualities
	if profile, err := GetAdaptiveProfile(Quality480p); err == nil {
		ladder = append(ladder, profile)
	}
	if profile, err := GetAdaptiveProfile(Quality720p2m); err == nil {
		ladder = append(ladder, profile)
	}
	if profile, err := GetAdaptiveProfile(Quality1080p4m); err == nil {
		ladder = append(ladder, profile)
	}

	// Add high quality if network allows
	if networkMbps >= 10 {
		if profile, err := GetAdaptiveProfile(Quality1080p10m); err == nil {
			ladder = append(ladder, profile)
		}
	}

	// Add 4K if screen and network support it
	if networkMbps >= 20 && screenHeight >= 2160 {
		if profile, err := GetAdaptiveProfile(Quality4k20m); err == nil {
			ladder = append(ladder, profile)
		}
	}

	qr.logger.Info("adaptive ladder generated",
		"rungs", len(ladder),
		"device", caps.DeviceType,
	)

	return ladder
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
