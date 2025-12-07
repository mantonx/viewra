package profile

import (
	"fmt"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// ABRLadder is the complete ABR ladder - single source of truth for all quality levels.
// This defines every quality variant available in the system.
var ABRLadder = []ABRVariant{
	// Low quality - emergency fallback / very poor connections
	{"360p", 800_000, 640, 360, "avc1.4d401e,mp4a.40.2"},

	// SD quality - poor mobile connections
	{"480p", 1_000_000, 854, 480, "avc1.4d401e,mp4a.40.2"},

	// HD quality - standard mobile/tablet
	{"720p-2m", 2_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
	{"720p-4m", 4_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},

	// Full HD - desktop/TV
	{"1080p-4m", 4_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
	{"1080p-10m", 10_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
	{"1080p-20m", 20_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
	{"1080p-40m", 40_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
	{"1080p-60m", 60_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},

	// 4K - high-end displays
	{"4k-15m", 15_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-20m", 20_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-25m", 25_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-40m", 40_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-60m", 60_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-100m", 100_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	{"4k-200m", 200_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
}

// abrLadderMap provides O(1) lookup by quality ID
var abrLadderMap = buildABRLadderMap()

func buildABRLadderMap() map[string]ABRVariant {
	m := make(map[string]ABRVariant, len(ABRLadder))
	for _, v := range ABRLadder {
		m[v.ID] = v
	}
	return m
}

// GetABRVariant returns the ABR variant for a given quality ID.
func GetABRVariant(qualityID string) (ABRVariant, bool) {
	v, ok := abrLadderMap[qualityID]
	return v, ok
}

// Quality profile IDs - these match the ABRLadder IDs exactly
const (
	// 360p - Low quality fallback
	Quality360p = "360p"

	// 480p - SD quality
	Quality480p = "480p"

	// 720p - HD quality
	Quality720p2m = "720p-2m"
	Quality720p4m = "720p-4m"

	// 1080p - Full HD
	Quality1080p4m  = "1080p-4m"
	Quality1080p10m = "1080p-10m"
	Quality1080p20m = "1080p-20m"
	Quality1080p40m = "1080p-40m"
	Quality1080p60m = "1080p-60m"

	// 4K - Ultra HD
	Quality4k15m  = "4k-15m"
	Quality4k20m  = "4k-20m"
	Quality4k25m  = "4k-25m"
	Quality4k40m  = "4k-40m"
	Quality4k60m  = "4k-60m"
	Quality4k100m = "4k-100m"
	Quality4k200m = "4k-200m"
)

// profileBuilder helps construct AdaptiveProfile instances with calculated defaults
type profileBuilder struct {
	profile *AdaptiveProfile
}

// newProfile creates a new profile builder with common defaults
func newProfile(id, displayName string, width, height, videoBitrate int) *profileBuilder {
	return &profileBuilder{
		profile: &AdaptiveProfile{
			ID:              id,
			DisplayName:     displayName,
			Width:           width,
			Height:          height,
			VideoBitrate:    videoBitrate,
			VideoMaxRate:    int(float64(videoBitrate) * 1.1), // 110% of target
			VideoBufSize:    videoBitrate * 2,                 // 2x target
			EnableHWAccel:   true,
			EnableFastStart: true,
			SegmentDuration: 2,
			GOPSize:         48,     // 2 seconds at 24fps (default)
			FrameRate:       24.0,   // Default frame rate
			AspectRatio:     "16:9", // Default aspect ratio
		},
	}
}

// withCodec sets the preferred codec
func (pb *profileBuilder) withCodec(codec string, fallbacks ...string) *profileBuilder {
	pb.profile.PreferredCodec = codec
	pb.profile.FallbackCodecs = fallbacks
	return pb
}

// withPreset sets the encoding preset and CRF
func (pb *profileBuilder) withPreset(preset string, crf int) *profileBuilder {
	pb.profile.Preset = preset
	pb.profile.CRF = crf
	return pb
}

// withNetwork sets network requirements
func (pb *profileBuilder) withNetwork(minMbps float64) *profileBuilder {
	pb.profile.MinNetworkMbps = minMbps
	return pb
}

// withScreen sets screen requirements
func (pb *profileBuilder) withScreen(minWidth, minHeight int) *profileBuilder {
	pb.profile.MinScreenWidth = minWidth
	pb.profile.MinScreenHeight = minHeight
	return pb
}

// withDevices sets recommended devices
func (pb *profileBuilder) withDevices(devices ...string) *profileBuilder {
	pb.profile.RecommendedFor = devices
	return pb
}

// withDescription sets description and quality tier
func (pb *profileBuilder) withDescription(desc, tier string) *profileBuilder {
	pb.profile.Description = desc
	pb.profile.QualityTier = tier
	return pb
}

// withFrameRate sets the frame rate and adjusts GOP size accordingly
func (pb *profileBuilder) withFrameRate(fps float64) *profileBuilder {
	pb.profile.FrameRate = fps
	pb.profile.GOPSize = int(fps * 2) // 2 seconds worth of frames
	return pb
}

// withAspectRatio sets aspect ratio and adjusts height if ultra-wide
func (pb *profileBuilder) withAspectRatio(ratio string) *profileBuilder {
	pb.profile.AspectRatio = ratio
	// Adjust height for ultra-wide formats
	if ratio == "2.39:1" && pb.profile.Width == 3840 {
		pb.profile.Height = 1606 // 3840 / 2.39
	}
	return pb
}

// with3D marks the profile as 3D with stereo mode
func (pb *profileBuilder) with3D(stereoMode string) *profileBuilder {
	pb.profile.Is3D = true
	pb.profile.StereoMode = stereoMode
	return pb
}

// build finalizes the profile and calculates data usage
func (pb *profileBuilder) build() *AdaptiveProfile {
	// Calculate data usage (MB per hour)
	totalBitrate := pb.profile.VideoBitrate + pb.profile.AudioBitrate
	pb.profile.DataUsageMBPerHour = (totalBitrate / 8) * 3600 / 1_000_000

	// Apply audio settings based on quality tier
	applyAudioSettings(pb.profile)

	return pb.profile
}

// buildProfiles constructs all adaptive profiles from the ABR ladder (single source of truth)
func buildProfiles() map[string]*AdaptiveProfile {
	profiles := make(map[string]*AdaptiveProfile)

	// Build profiles from the ABR ladder
	for _, variant := range ABRLadder {
		profile := buildProfileFromVariant(variant)
		profiles[variant.ID] = profile
	}

	return profiles
}

// buildProfileFromVariant creates an AdaptiveProfile from an ABRVariant
func buildProfileFromVariant(v ABRVariant) *AdaptiveProfile {
	// Determine quality tier and settings based on resolution and bitrate
	var tier, desc, preset string
	var crf int
	var minNetwork float64
	var devices []string

	switch {
	case v.Height <= 360:
		tier, desc = "low", "Low quality fallback for poor connections"
		preset, crf = "fast", 26
		minNetwork = 1.0
		devices = []string{"mobile"}
	case v.Height <= 480:
		tier, desc = "medium", "SD quality for mobile"
		preset, crf = "medium", 24
		minNetwork = 1.5
		devices = []string{"mobile", "tablet"}
	case v.Height <= 720:
		if v.Bandwidth <= 2_500_000 {
			tier, desc = "high", "HD quality for mobile/tablet"
			preset, crf = "medium", 23
			minNetwork = 2.5
			devices = []string{"mobile", "tablet"}
		} else {
			tier, desc = "high", "HD quality for tablet/desktop"
			preset, crf = "medium", 22
			minNetwork = 5.0
			devices = []string{"tablet", "desktop"}
		}
	case v.Height <= 1080:
		if v.Bandwidth <= 5_000_000 {
			tier, desc = "high", "Entry-level Full HD"
			preset, crf = "medium", 23
			minNetwork = 5.0
			devices = []string{"tablet", "desktop"}
		} else if v.Bandwidth <= 15_000_000 {
			tier, desc = "high", "Standard Full HD"
			preset, crf = "medium", 21
			minNetwork = float64(v.Bandwidth) / 800_000 // ~1.25x headroom
			devices = []string{"desktop", "tv"}
		} else if v.Bandwidth <= 25_000_000 {
			tier, desc = "ultra", "High quality Full HD"
			preset, crf = "medium", 19
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"desktop", "tv"}
		} else if v.Bandwidth <= 50_000_000 {
			tier, desc = "ultra", "Premium Full HD"
			preset, crf = "slow", 17
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		} else {
			tier, desc = "ultra", "Reference Full HD quality"
			preset, crf = "slow", 16
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		}
	default: // 4K
		if v.Bandwidth <= 20_000_000 {
			tier, desc = "ultra", "Entry-level 4K quality"
			preset, crf = "medium", 22
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"desktop", "tv"}
		} else if v.Bandwidth <= 30_000_000 {
			tier, desc = "ultra", "Standard 4K quality"
			preset, crf = "medium", 21
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		} else if v.Bandwidth <= 50_000_000 {
			tier, desc = "ultra", "High quality 4K"
			preset, crf = "medium", 20
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		} else if v.Bandwidth <= 80_000_000 {
			tier, desc = "ultra", "Premium 4K quality"
			preset, crf = "medium", 18
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		} else if v.Bandwidth <= 150_000_000 {
			tier, desc = "ultra", "Ultra premium 4K quality"
			preset, crf = "slow", 16
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		} else {
			tier, desc = "ultra", "Reference 4K quality"
			preset, crf = "slow", 14
			minNetwork = float64(v.Bandwidth) / 800_000
			devices = []string{"tv"}
		}
	}

	// Create display name from ID
	displayName := fmt.Sprintf("%dp (%d Mbps)", v.Height, v.Bandwidth/1_000_000)
	if v.Height >= 2160 {
		displayName = fmt.Sprintf("4K (%d Mbps)", v.Bandwidth/1_000_000)
	}

	return newProfile(v.ID, displayName, v.Width, v.Height, v.Bandwidth).
		withCodec("h264").
		withPreset(preset, crf).
		withNetwork(minNetwork).
		withScreen(v.Width, v.Height).
		withDevices(devices...).
		withDescription(desc, tier).
		build()
}

// adaptiveProfiles is initialized using the builder pattern
var adaptiveProfiles = buildProfiles()

// GetAdaptiveProfile returns the adaptive profile for a given quality ID.
func GetAdaptiveProfile(quality string) (*AdaptiveProfile, error) {
	profile, exists := adaptiveProfiles[quality]
	if !exists {
		return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
	}
	return profile, nil
}

// GetAllAdaptiveProfiles returns all available adaptive quality profiles.
func GetAllAdaptiveProfiles() []*AdaptiveProfile {
	profiles := make([]*AdaptiveProfile, 0, len(adaptiveProfiles))
	for _, profile := range adaptiveProfiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// IsAdaptiveQualitySupported checks if an adaptive quality level is supported.
func IsAdaptiveQualitySupported(quality string) bool {
	_, exists := adaptiveProfiles[quality]
	return exists
}

// GetAdaptiveQualitiesByTier returns profiles filtered by quality tier.
func GetAdaptiveQualitiesByTier(tier string) []*AdaptiveProfile {
	filtered := []*AdaptiveProfile{}
	for _, profile := range adaptiveProfiles {
		if profile.QualityTier == tier {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

// FilterProfilesByScreenSize filters profiles that fit within screen dimensions.
func FilterProfilesByScreenSize(profiles []*AdaptiveProfile, width, height int) []*AdaptiveProfile {
	filtered := []*AdaptiveProfile{}
	for _, profile := range profiles {
		if profile.Width <= width && profile.Height <= height {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

// FilterProfilesByNetworkSpeed filters profiles that work with given network speed.
func FilterProfilesByNetworkSpeed(profiles []*AdaptiveProfile, speedMbps float64) []*AdaptiveProfile {
	filtered := []*AdaptiveProfile{}
	for _, profile := range profiles {
		if profile.MinNetworkMbps <= speedMbps {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

// GetAdaptiveProfileForQuality maps quality strings to AdaptiveProfile instances.
// Quality IDs now match the ABR ladder directly (e.g., "4k-60m", "1080p-20m", "720p-4m").
// Also supports legacy format: "360p", "720p", "1080p", "4k"
func GetAdaptiveProfileForQuality(quality string) (*AdaptiveProfile, error) {
	// Direct lookup - IDs match ABR ladder exactly
	if profile, exists := adaptiveProfiles[quality]; exists {
		return profile, nil
	}

	// Fall back to legacy quality mapping (simple resolution names)
	var profileID string
	switch quality {
	case transcode.Quality360p:
		profileID = Quality360p
	case transcode.Quality480p:
		profileID = Quality480p
	case transcode.Quality720p:
		profileID = Quality720p4m
	case transcode.Quality1080p:
		profileID = Quality1080p10m
	case transcode.Quality4K:
		profileID = Quality4k60m
	default:
		return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
	}
	return GetAdaptiveProfile(profileID)
}

// applyAudioSettings configures audio parameters based on quality tier and resolution.
// This centralizes audio configuration logic to avoid repetition.
func applyAudioSettings(profile *AdaptiveProfile) {
	switch profile.QualityTier {
	case "low":
		// 240p-360p: Stereo only, AAC for maximum compatibility
		profile.AudioBitrate = 64_000
		profile.AudioChannels = 2
		profile.AudioSampleRate = 44100
		profile.PreserveMultiCh = false
		profile.AudioCodec = "aac"
		profile.MaxAudioChannels = 2

	case "medium":
		// 480p: Stereo with higher bitrate, preserve up to stereo
		profile.AudioBitrate = 128_000
		profile.AudioChannels = 2
		profile.AudioSampleRate = 48000
		profile.PreserveMultiCh = false
		profile.AudioCodec = "aac"
		profile.MaxAudioChannels = 2

	case "high":
		// 720p-1080p: Preserve 5.1 surround, use AC3/AAC
		if profile.Width >= 1280 {
			profile.AudioBitrate = 256_000
			profile.AudioChannels = 6 // 5.1
			profile.AudioSampleRate = 48000
			profile.PreserveMultiCh = true
			profile.AudioCodec = "ac3" // Dolby Digital 5.1
			profile.MaxAudioChannels = 6
		} else {
			// 720p lower bitrates
			profile.AudioBitrate = 192_000
			profile.AudioChannels = 2
			profile.AudioSampleRate = 48000
			profile.PreserveMultiCh = true
			profile.AudioCodec = "aac"
			profile.MaxAudioChannels = 6
		}

	case "ultra":
		// 1440p-4K: Preserve all channels (7.1, Atmos), use EAC3
		profile.AudioBitrate = 320_000
		profile.AudioChannels = 8 // 7.1
		profile.AudioSampleRate = 48000
		profile.PreserveMultiCh = true
		profile.AudioCodec = "eac3"  // Dolby Digital Plus, supports Atmos
		profile.MaxAudioChannels = 0 // No limit, preserve all channels
	}
}
