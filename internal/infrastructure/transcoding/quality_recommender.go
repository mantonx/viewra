package transcoding

import (
	"log/slog"
	"sort"
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

	// Performance
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
func (qr *QualityRecommender) RecommendQuality(caps ClientCapabilities) *QualityRecommendation {
	recommendations := qr.RecommendMultipleQualities(caps, 1)
	if len(recommendations) == 0 {
		// Fallback to safest profile
		profile, _ := GetAdaptiveProfile(Quality480p1200k)
		return &QualityRecommendation{
			Profile: profile,
			Score:   0.5,
			Reason:  "fallback to safe default",
		}
	}
	return recommendations[0]
}

// RecommendMultipleQualities returns top N recommended quality profiles.
func (qr *QualityRecommender) RecommendMultipleQualities(caps ClientCapabilities, count int) []*QualityRecommendation {
	allProfiles := GetAllAdaptiveProfiles()
	recommendations := make([]*QualityRecommendation, 0, len(allProfiles))

	qr.logger.Info("starting quality recommendation",
		"network_mbps", caps.NetworkSpeedMbps,
		"device", caps.DeviceType,
		"screen_width", caps.ScreenWidth,
		"screen_height", caps.ScreenHeight,
		"supported_codecs", caps.SupportedCodecs,
		"total_profiles", len(allProfiles),
	)

	for _, profile := range allProfiles {
		score, reason := qr.scoreProfile(profile, caps)
		if score > 0 {
			recommendations = append(recommendations, &QualityRecommendation{
				Profile: profile,
				Score:   score,
				Reason:  reason,
			})
		} else {
			// Log why profile was rejected (only for 1080p+ profiles to reduce noise)
			if profile.Height >= 1080 {
				qr.logger.Debug("profile rejected",
					"profile", profile.ID,
					"height", profile.Height,
					"reason", reason,
				)
			}
		}
	}

	// Sort by score (descending)
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// Return top N
	if len(recommendations) > count {
		recommendations = recommendations[:count]
	}

	// Log top recommendations
	if len(recommendations) > 0 {
		top := recommendations[0]
		qr.logger.Info("quality recommendation result",
			"top_profile", top.Profile.ID,
			"top_score", top.Score,
			"top_height", top.Profile.Height,
			"total_valid", len(recommendations),
		)
	} else {
		qr.logger.Warn("no valid profiles found, will use fallback")
	}

	return recommendations
}

// scoreProfile calculates a compatibility score (0-1) for a profile given client capabilities.
func (qr *QualityRecommender) scoreProfile(profile *AdaptiveProfile, caps ClientCapabilities) (float64, string) {
	score := 1.0
	reasons := []string{}

	// 1. Network speed compatibility (critical)
	networkScore := qr.scoreNetwork(profile, caps)
	if networkScore == 0 {
		return 0, "insufficient network speed"
	}
	score *= networkScore

	// 2. Screen resolution compatibility (important)
	screenScore := qr.scoreScreen(profile, caps)
	if screenScore < 0.3 {
		return 0, "screen resolution mismatch"
	}
	score *= screenScore

	// 3. Device type compatibility (moderate)
	deviceScore := qr.scoreDeviceType(profile, caps)
	score *= deviceScore

	// 4. Codec compatibility (critical)
	codecScore := qr.scoreCodec(profile, caps)
	if codecScore == 0 {
		return 0, "codec not supported"
	}
	score *= codecScore

	// 5. Power considerations (moderate)
	powerScore := qr.scorePower(profile, caps)
	score *= powerScore

	// 6. Metered connection penalty (significant)
	if caps.IsMetered && profile.MinNetworkMbps > 5.0 {
		score *= 0.5
		reasons = append(reasons, "metered connection penalty")
	}

	// Build reason string
	reason := "optimal match"
	if len(reasons) > 0 {
		reason = reasons[0]
	}

	return score, reason
}

// scoreNetwork evaluates network speed compatibility.
func (qr *QualityRecommender) scoreNetwork(profile *AdaptiveProfile, caps ClientCapabilities) float64 {
	requiredMbps := profile.MinNetworkMbps
	availableMbps := caps.NetworkSpeedMbps

	// If network speed is unknown/failed (-1 or 0), assume a decent connection
	// This prevents defaulting to lowest quality when measurement fails
	// Actual quality will adapt during playback based on real throughput
	if availableMbps <= 0 {
		availableMbps = 50.0 // Assume 50 Mbps as reasonable default
	}

	// Need at least the minimum
	if availableMbps < requiredMbps {
		return 0
	}

	// Buffer factor: prefer profiles that use 60-80% of available bandwidth
	usageRatio := requiredMbps / availableMbps

	if usageRatio <= 0.6 {
		// Under-utilizing bandwidth, but not a dealbreaker
		return 0.8
	} else if usageRatio <= 0.8 {
		// Optimal range: 60-80% utilization
		return 1.0
	} else if usageRatio <= 0.95 {
		// Close to max, but acceptable with buffer
		return 0.9
	} else {
		// Too close to limit, potential buffering
		return 0.7
	}
}

// scoreScreen evaluates screen resolution compatibility.
func (qr *QualityRecommender) scoreScreen(profile *AdaptiveProfile, caps ClientCapabilities) float64 {
	screenPixels := caps.ScreenWidth * caps.ScreenHeight
	profilePixels := profile.Width * profile.Height

	ratio := float64(profilePixels) / float64(screenPixels)

	if ratio <= 0.5 {
		// Significant downscaling - not ideal but acceptable
		return 0.7
	} else if ratio <= 1.0 {
		// Perfect fit or minor downscaling
		return 1.0
	} else if ratio <= 1.5 {
		// Minor upscaling - still good on high DPI screens
		return 0.95
	} else if ratio <= 2.0 {
		// Moderate upscaling - may look soft
		return 0.6
	} else {
		// Excessive upscaling - wasteful
		return 0.3
	}
}

// scoreDeviceType evaluates device type compatibility.
func (qr *QualityRecommender) scoreDeviceType(profile *AdaptiveProfile, caps ClientCapabilities) float64 {
	// Check if device type is in recommended list
	for _, recommendedDevice := range profile.RecommendedFor {
		if recommendedDevice == caps.DeviceType {
			return 1.0
		}
	}

	// Not explicitly recommended, but not disqualifying
	return 0.8
}

// scoreCodec evaluates codec compatibility.
func (qr *QualityRecommender) scoreCodec(profile *AdaptiveProfile, caps ClientCapabilities) float64 {
	// If no codecs reported, assume h264 is supported (universal compatibility)
	// This handles cases where codec detection failed on the client
	supportedCodecs := caps.SupportedCodecs
	if len(supportedCodecs) == 0 {
		supportedCodecs = []string{"h264"}
	}

	// Check if preferred codec is supported
	for _, supportedCodec := range supportedCodecs {
		if supportedCodec == profile.PreferredCodec {
			return 1.0
		}
	}

	// Check if any fallback codec is supported
	for _, fallbackCodec := range profile.FallbackCodecs {
		for _, supportedCodec := range supportedCodecs {
			if supportedCodec == fallbackCodec {
				return 0.9 // Slightly lower score for fallback
			}
		}
	}

	// No compatible codec
	return 0
}

// scorePower evaluates power/battery considerations.
func (qr *QualityRecommender) scorePower(profile *AdaptiveProfile, caps ClientCapabilities) float64 {
	score := 1.0

	// Low power mode penalty for high-quality profiles
	if caps.LowPowerMode {
		switch profile.QualityTier {
		case "ultra":
			score *= 0.5
		case "high":
			score *= 0.8
		}
	}

	// Low battery penalty for high bitrates (only if not charging)
	if !caps.IsCharging && caps.BatteryLevel > 0 && caps.BatteryLevel < 0.2 {
		if profile.VideoBitrate > 10_000_000 { // > 10 Mbps
			score *= 0.6
		}
	}

	// Hardware acceleration bonus for high-res profiles
	if caps.HardwareAcceleration && (profile.Width >= 1920 || profile.FrameRate >= 60) {
		score *= 1.1 // 10% bonus
	}

	return score
}

// GetAdaptiveLadder returns a set of quality profiles for adaptive streaming (ABR ladder).
func (qr *QualityRecommender) GetAdaptiveLadder(caps ClientCapabilities) []*AdaptiveProfile {
	// Get all compatible profiles
	allRecommendations := qr.RecommendMultipleQualities(caps, 100)

	// Build ladder with diverse quality levels
	ladder := make([]*AdaptiveProfile, 0, 6)

	// Always include lowest quality for poor conditions
	lowest, _ := GetAdaptiveProfile(Quality240p400k)
	ladder = append(ladder, lowest)

	// Include profiles at different bitrate levels
	targetBitrates := []int{
		1_200_000,  // ~1.2 Mbps (SD)
		2_500_000,  // ~2.5 Mbps (SD high)
		5_000_000,  // ~5 Mbps (720p)
		8_000_000,  // ~8 Mbps (1080p)
		16_000_000, // ~16 Mbps (1080p high / 1440p)
	}

	for _, targetBitrate := range targetBitrates {
		closestProfile := qr.findClosestProfile(allRecommendations, targetBitrate)
		if closestProfile != nil && !qr.isInLadder(ladder, closestProfile) {
			ladder = append(ladder, closestProfile)
		}
	}

	// Add best quality profile as top rung
	if len(allRecommendations) > 0 && !qr.isInLadder(ladder, allRecommendations[0].Profile) {
		ladder = append(ladder, allRecommendations[0].Profile)
	}

	qr.logger.Info("adaptive ladder generated",
		"rungs", len(ladder),
		"device", caps.DeviceType,
	)

	return ladder
}

// findClosestProfile finds the profile closest to target bitrate.
func (qr *QualityRecommender) findClosestProfile(recommendations []*QualityRecommendation, targetBitrate int) *AdaptiveProfile {
	var closest *AdaptiveProfile
	minDiff := int(^uint(0) >> 1) // max int

	for _, rec := range recommendations {
		diff := rec.Profile.VideoBitrate - targetBitrate
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			closest = rec.Profile
		}
	}

	return closest
}

// isInLadder checks if a profile is already in the ladder.
func (qr *QualityRecommender) isInLadder(ladder []*AdaptiveProfile, profile *AdaptiveProfile) bool {
	for _, p := range ladder {
		if p.ID == profile.ID {
			return true
		}
	}
	return false
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
		"device", caps.DeviceType,
		"low_power", caps.LowPowerMode,
	)

	// For low power mode or metered connections, prefer more efficient codecs
	preferEfficiency := caps.LowPowerMode || caps.IsMetered

	// Check codecs in order of compression efficiency
	// AV1 > H.265/VP9 > H.264

	// Try AV1 (best compression, 30% better than H.265)
	if supported["av1"] && IsCodecSupported(hwAccel, CodecAV1) {
		qr.logger.Info("recommending codec", "codec", "av1", "reason", "best compression")
		return CodecAV1
	}

	// Try H.265 (25% better compression than H.264, good browser support)
	if supported["h265"] && IsCodecSupported(hwAccel, CodecH265) {
		// H.265 is great for Safari and Edge
		qr.logger.Info("recommending codec", "codec", "h265", "reason", "good compression, Safari/Edge support")
		return CodecH265
	}

	// Try VP9 (similar to H.265, Chrome/Firefox preferred)
	if supported["vp9"] && IsCodecSupported(hwAccel, CodecVP9) {
		// VP9 is royalty-free and well-supported in Chrome/Firefox
		qr.logger.Info("recommending codec", "codec", "vp9", "reason", "royalty-free, Chrome/Firefox support")
		return CodecVP9
	}

	// Fall back to H.264 (universal compatibility)
	// Note: preferEfficiency could be used for future enhancement where we might
	// weight codecs differently based on power constraints
	_ = preferEfficiency
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
