package streaming

// ProfileProvider provides access to streaming quality profiles.
type ProfileProvider interface {
	// GetProfile returns the adaptive profile for a given quality ID.
	GetProfile(qualityID string) (*AdaptiveProfile, error)

	// GetAllProfiles returns all available adaptive quality profiles.
	GetAllProfiles() []*AdaptiveProfile

	// IsQualitySupported checks if a quality level is supported.
	IsQualitySupported(qualityID string) bool

	// GetProfilesByTier returns profiles filtered by quality tier.
	GetProfilesByTier(tier string) []*AdaptiveProfile
}

// QualityRecommender recommends optimal quality profiles based on client capabilities.
type QualityRecommender interface {
	// RecommendQuality recommends the best quality profile for given client capabilities.
	RecommendQuality(caps ClientCapabilities) *QualityRecommendation

	// RecommendMultipleQualities returns top N recommended quality profiles.
	RecommendMultipleQualities(caps ClientCapabilities, count int) []*QualityRecommendation

	// GetAdaptiveLadder returns a set of quality profiles for adaptive streaming (ABR ladder).
	GetAdaptiveLadder(caps ClientCapabilities) []*AdaptiveProfile
}

// ABRLadderProvider provides access to the ABR (Adaptive Bitrate) ladder.
type ABRLadderProvider interface {
	// GetVariant returns the ABR variant for a given quality ID.
	GetVariant(qualityID string) (ABRVariant, bool)

	// GetAllVariants returns all variants in the ABR ladder.
	GetAllVariants() []ABRVariant
}
