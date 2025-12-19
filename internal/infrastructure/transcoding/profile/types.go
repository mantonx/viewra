// Package profile provides quality profile definitions and recommendations for adaptive streaming.
//
// The profile package is organized as follows:
//   - types.go: Type aliases from domain/streaming (this file)
//   - adaptive.go: AdaptiveProfile building, ABR ladder, profile lookup
//   - recommender.go: QualityRecommender for client-based quality selection
package profile

import (
	"github.com/mantonx/viewra/internal/domain/streaming"
)

// Type aliases from domain/streaming - allows existing code to continue using profile.X

// AdaptiveProfile defines granular bitrate-based quality profiles for adaptive streaming.
type AdaptiveProfile = streaming.AdaptiveProfile

// QualityVariant represents a single quality variant in the quality ladder.
type QualityVariant = streaming.QualityVariant

// ABRVariant is an alias for QualityVariant for backwards compatibility.
// Deprecated: Use QualityVariant instead.
type ABRVariant = streaming.QualityVariant

// ClientCapabilities represents the detected capabilities of a client device.
type ClientCapabilities = streaming.ClientCapabilities

// QualityRecommendation represents a recommended quality profile with score.
type QualityRecommendation = streaming.QualityRecommendation

// Re-export quality constants from domain/streaming
const (
	// 360p - Low quality fallback
	Quality360p = streaming.Quality360p

	// 480p - SD quality
	Quality480p = streaming.Quality480p

	// 720p - HD quality
	Quality720p2m = streaming.Quality720p2m
	Quality720p4m = streaming.Quality720p4m

	// 1080p - Full HD
	Quality1080p4m  = streaming.Quality1080p4m
	Quality1080p10m = streaming.Quality1080p10m
	Quality1080p20m = streaming.Quality1080p20m
	Quality1080p40m = streaming.Quality1080p40m
	Quality1080p60m = streaming.Quality1080p60m

	// 4K - Ultra HD
	Quality4k15m  = streaming.Quality4k15m
	Quality4k20m  = streaming.Quality4k20m
	Quality4k25m  = streaming.Quality4k25m
	Quality4k40m  = streaming.Quality4k40m
	Quality4k60m  = streaming.Quality4k60m
	Quality4k100m = streaming.Quality4k100m
	Quality4k200m = streaming.Quality4k200m
)

// Re-export quality tier constants
const (
	QualityTierLow    = streaming.QualityTierLow
	QualityTierMedium = streaming.QualityTierMedium
	QualityTierHigh   = streaming.QualityTierHigh
	QualityTierUltra  = streaming.QualityTierUltra
)

// Re-export device type constants
const (
	DeviceTypeMobile  = streaming.DeviceTypeMobile
	DeviceTypeTablet  = streaming.DeviceTypeTablet
	DeviceTypeDesktop = streaming.DeviceTypeDesktop
	DeviceTypeTV      = streaming.DeviceTypeTV
)
