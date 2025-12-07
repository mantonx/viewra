package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// AdaptiveProfile is re-exported from the profile subpackage for backward compatibility.
type AdaptiveProfile = profile.AdaptiveProfile

// ABRVariant is re-exported from the profile subpackage for backward compatibility.
type ABRVariant = profile.ABRVariant

// ABRLadder is re-exported from the profile subpackage.
// This is the complete ABR ladder - single source of truth for all quality levels.
var ABRLadder = profile.ABRLadder

// Quality profile IDs - re-exported from profile subpackage
const (
	// 360p - Low quality fallback
	Quality360p = profile.Quality360p

	// 480p - SD quality
	Quality480p = profile.Quality480p

	// 720p - HD quality
	Quality720p2m = profile.Quality720p2m
	Quality720p4m = profile.Quality720p4m

	// 1080p - Full HD
	Quality1080p4m  = profile.Quality1080p4m
	Quality1080p10m = profile.Quality1080p10m
	Quality1080p20m = profile.Quality1080p20m
	Quality1080p40m = profile.Quality1080p40m
	Quality1080p60m = profile.Quality1080p60m

	// 4K - Ultra HD
	Quality4k15m  = profile.Quality4k15m
	Quality4k20m  = profile.Quality4k20m
	Quality4k25m  = profile.Quality4k25m
	Quality4k40m  = profile.Quality4k40m
	Quality4k60m  = profile.Quality4k60m
	Quality4k100m = profile.Quality4k100m
	Quality4k200m = profile.Quality4k200m
)

// GetABRVariant returns the ABR variant for a given quality ID.
// Re-exported from profile subpackage for backward compatibility.
func GetABRVariant(qualityID string) (ABRVariant, bool) {
	return profile.GetABRVariant(qualityID)
}

// GetAdaptiveProfile returns the adaptive profile for a given quality ID.
// Re-exported from profile subpackage for backward compatibility.
func GetAdaptiveProfile(quality string) (*AdaptiveProfile, error) {
	return profile.GetAdaptiveProfile(quality)
}

// GetAllAdaptiveProfiles returns all available adaptive quality profiles.
// Re-exported from profile subpackage for backward compatibility.
func GetAllAdaptiveProfiles() []*AdaptiveProfile {
	return profile.GetAllAdaptiveProfiles()
}

// IsAdaptiveQualitySupported checks if an adaptive quality level is supported.
// Re-exported from profile subpackage for backward compatibility.
func IsAdaptiveQualitySupported(quality string) bool {
	return profile.IsAdaptiveQualitySupported(quality)
}

// GetAdaptiveQualitiesByTier returns profiles filtered by quality tier.
// Re-exported from profile subpackage for backward compatibility.
func GetAdaptiveQualitiesByTier(tier string) []*AdaptiveProfile {
	return profile.GetAdaptiveQualitiesByTier(tier)
}

// FilterProfilesByScreenSize filters profiles that fit within screen dimensions.
// Re-exported from profile subpackage for backward compatibility.
func FilterProfilesByScreenSize(profiles []*AdaptiveProfile, width, height int) []*AdaptiveProfile {
	return profile.FilterProfilesByScreenSize(profiles, width, height)
}

// FilterProfilesByNetworkSpeed filters profiles that work with given network speed.
// Re-exported from profile subpackage for backward compatibility.
func FilterProfilesByNetworkSpeed(profiles []*AdaptiveProfile, speedMbps float64) []*AdaptiveProfile {
	return profile.FilterProfilesByNetworkSpeed(profiles, speedMbps)
}

// GetAdaptiveProfileForQuality maps quality strings to AdaptiveProfile instances.
// Re-exported from profile subpackage for backward compatibility.
func GetAdaptiveProfileForQuality(quality string) (*AdaptiveProfile, error) {
	return profile.GetAdaptiveProfileForQuality(quality)
}
