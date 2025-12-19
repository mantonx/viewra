package profile

import (
	"log/slog"
)

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

	// Network-first selection: prioritize bandwidth over screen resolution
	// For remux scenarios (no transcode cost), higher quality doesn't waste server resources
	// Screen resolution is used as a secondary hint, not a hard cap
	var profileID string
	var reason string

	// Determine effective resolution: use screen height but allow higher for desktop/TV with good network
	effectiveHeight := screenHeight
	if (caps.DeviceType == "desktop" || caps.DeviceType == "tv" || caps.DeviceType == "") && networkMbps >= 25 {
		// Desktop/TV users with good network likely have high-quality displays
		// Allow 4K even on 1440p screens - the extra resolution helps with HDR content
		if screenHeight >= 1080 {
			effectiveHeight = 2160
		}
	}

	switch {
	// 4K selection - prioritize network, effective height allows desktop/TV upgrade
	case networkMbps >= 60 && effectiveHeight >= 2160:
		profileID = Quality4k60m
		reason = "excellent network (4K capable)"
	case networkMbps >= 40 && effectiveHeight >= 2160:
		profileID = Quality4k40m
		reason = "good network (4K capable)"
	case networkMbps >= 25 && effectiveHeight >= 2160:
		profileID = Quality4k25m
		reason = "moderate network (4K capable)"
	case networkMbps >= 15 && effectiveHeight >= 2160:
		profileID = Quality4k15m
		reason = "acceptable network (4K capable)"

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
