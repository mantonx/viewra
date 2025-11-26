package transcoding

import (
	"fmt"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// AdaptiveProfile defines granular bitrate-based quality profiles for adaptive streaming.
// Format: "{resolution}-{bitrate}" (e.g., "1080p-8000k", "4k-40000k")
// This gives users precise control over bandwidth vs quality tradeoff.
type AdaptiveProfile struct {
	// Identity
	ID          string // "1080p-8000k", "4k-40000k"
	DisplayName string // "1080p High (8 Mbps)", "4K Ultra (40 Mbps)"

	// Resolution
	Width  int
	Height int

	// Bitrate (specific values, not ranges)
	VideoBitrate int // bits per second (e.g., 8_000_000 for 8 Mbps)
	VideoMaxRate int // 110% of target for VBV
	VideoBufSize int // 2x target for VBV

	// Audio
	AudioBitrate     int // bits per second
	AudioChannels    int // Target channels (2=stereo, 6=5.1, 8=7.1)
	AudioSampleRate  int
	PreserveMultiCh  bool   // If true, keep original multi-channel audio (no downmix)
	AudioCodec       string // Target audio codec: "aac", "ac3", "eac3", "opus"
	MaxAudioChannels int    // Maximum channels to preserve (0 = no limit)

	// Codec preferences
	PreferredCodec string   // "h264", "h265", "vp9", "av1"
	FallbackCodecs []string // For Phase 3 multi-codec support

	// Encoding parameters
	Preset          string // "ultrafast", "fast", "medium", "slow", "veryslow"
	CRF             int    // Constant Rate Factor (quality: 15-28)
	EnableHWAccel   bool
	EnableFastStart bool

	// HLS segments
	SegmentDuration int
	GOPSize         int

	// Frame rate and aspect ratio
	FrameRate   float64 // Target frame rate (24, 30, 60)
	AspectRatio string  // "16:9", "21:9", "2.39:1", etc.
	Is3D        bool    // Is this a 3D profile
	StereoMode  string  // "sbs" (side-by-side), "tab" (top-and-bottom), "" (2D)

	// Client requirements
	MinNetworkMbps  float64
	MinScreenWidth  int
	MinScreenHeight int
	RecommendedFor  []string // ["desktop", "tablet", "mobile", "tv"]

	// Metadata
	DataUsageMBPerHour int
	Description        string
	QualityTier        string // "low", "medium", "high", "ultra"
}

// Granular bitrate-based quality profile IDs (23 total profiles)
const (
	// 240p - Ultra Low
	Quality240p400k = "240p-400k"

	// 360p - Low
	Quality360p800k = "360p-800k"

	// 480p - Standard Definition (3 variants)
	Quality480p1200k = "480p-1200k"
	Quality480p1800k = "480p-1800k"
	Quality480p2500k = "480p-2500k"

	// 720p - HD (4 variants)
	Quality720p2500k = "720p-2500k"
	Quality720p4000k = "720p-4000k"
	Quality720p5500k = "720p-5500k"
	Quality720p7500k = "720p-7500k"

	// 1080p - Full HD (5 variants)
	Quality1080p4000k  = "1080p-4000k"
	Quality1080p6000k  = "1080p-6000k"
	Quality1080p8000k  = "1080p-8000k"
	Quality1080p12000k = "1080p-12000k"
	Quality1080p16000k = "1080p-16000k"

	// 1440p - 2K (4 variants)
	Quality1440p8000k  = "1440p-8000k"
	Quality1440p12000k = "1440p-12000k"
	Quality1440p16000k = "1440p-16000k"
	Quality1440p24000k = "1440p-24000k"

	// 4K - Ultra HD (7 variants)
	Quality4k16000k  = "4k-16000k"
	Quality4k25000k  = "4k-25000k"
	Quality4k35000k  = "4k-35000k"
	Quality4k50000k  = "4k-50000k"
	Quality4k80000k  = "4k-80000k"
	Quality4k100000k = "4k-100000k"
	Quality4k120000k = "4k-120000k"

	// High Frame Rate (60 FPS) - 4 variants
	Quality720p8000k60fps   = "720p-8000k-60fps"
	Quality1080p12000k60fps = "1080p-12000k-60fps"
	Quality1080p20000k60fps = "1080p-20000k-60fps"
	Quality4k50000k60fps    = "4k-50000k-60fps"

	// 3D Variants (Top-and-Bottom) - 3 variants
	Quality1080p12000k3d = "1080p-12000k-3d"
	Quality1080p16000k3d = "1080p-16000k-3d"
	Quality4k50000k3d    = "4k-50000k-3d"

	// Ultra-wide (2.39:1) - 2 variants
	Quality4k25000kWide = "4k-25000k-wide"
	Quality4k80000kWide = "4k-80000k-wide"
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

// buildProfiles constructs all adaptive profiles using the builder pattern
func buildProfiles() map[string]*AdaptiveProfile {
	return map[string]*AdaptiveProfile{
		// 240p - Ultra Low (Poor connections, data saving)
		Quality240p400k: newProfile(Quality240p400k, "240p Ultra Low (0.4 Mbps)", 426, 240, 400_000).
			withCodec("h264").
			withPreset("fast", 28).
			withNetwork(0.5).
			withScreen(320, 240).
			withDevices("mobile").
			withDescription("Minimum quality for very poor connections", "low").
			build(),

		// 360p - Low
		Quality360p800k: newProfile(Quality360p800k, "360p Low (0.8 Mbps)", 640, 360, 800_000).
			withCodec("h264").
			withPreset("fast", 26).
			withNetwork(1.0).
			withScreen(480, 320).
			withDevices("mobile").
			withDescription("Basic quality for mobile devices", "low").
			build(),

		// 480p - Standard Definition (3 variants)
		Quality480p1200k: newProfile(Quality480p1200k, "480p Low (1.2 Mbps)", 854, 480, 1_200_000).
			withCodec("h264").
			withPreset("medium", 24).
			withNetwork(1.5).
			withScreen(640, 480).
			withDevices("mobile", "tablet").
			withDescription("Entry-level SD quality", "medium").
			build(),

		Quality480p1800k: newProfile(Quality480p1800k, "480p Medium (1.8 Mbps)", 854, 480, 1_800_000).
			withCodec("h264").
			withPreset("medium", 23).
			withNetwork(2.2).
			withScreen(640, 480).
			withDevices("mobile", "tablet").
			withDescription("Balanced SD quality", "medium").
			build(),

		Quality480p2500k: newProfile(Quality480p2500k, "480p High (2.5 Mbps)", 854, 480, 2_500_000).
			withCodec("h264").
			withPreset("medium", 22).
			withNetwork(3.0).
			withScreen(640, 480).
			withDevices("tablet", "desktop").
			withDescription("High-quality SD", "medium").
			build(),

		// 720p - HD (4 variants)
		Quality720p2500k: newProfile(Quality720p2500k, "720p Low (2.5 Mbps)", 1280, 720, 2_500_000).
			withCodec("h264").
			withPreset("medium", 23).
			withNetwork(3.0).
			withScreen(1280, 720).
			withDevices("tablet", "desktop").
			withDescription("Entry-level HD", "high").
			build(),

		Quality720p4000k: newProfile(Quality720p4000k, "720p Medium (4 Mbps)", 1280, 720, 4_000_000).
			withCodec("h264").
			withPreset("medium", 22).
			withNetwork(5.0).
			withScreen(1280, 720).
			withDevices("tablet", "desktop", "tv").
			withDescription("Balanced HD quality", "high").
			build(),

		Quality720p5500k: newProfile(Quality720p5500k, "720p High (5.5 Mbps)", 1280, 720, 5_500_000).
			withCodec("h264").
			withPreset("medium", 21).
			withNetwork(7.0).
			withScreen(1280, 720).
			withDevices("desktop", "tv").
			withDescription("High-quality HD", "high").
			build(),

		Quality720p7500k: newProfile(Quality720p7500k, "720p Ultra (7.5 Mbps)", 1280, 720, 7_500_000).
			withCodec("h264").
			withPreset("slow", 20).
			withNetwork(9.0).
			withScreen(1280, 720).
			withDevices("desktop", "tv").
			withDescription("Maximum 720p quality", "high").
			build(),

		// 1080p - Full HD (5 variants)
		Quality1080p4000k: newProfile(Quality1080p4000k, "1080p Low (4 Mbps)", 1920, 1080, 4_000_000).
			withCodec("h264").
			withPreset("medium", 23).
			withNetwork(5.0).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withDescription("Entry-level Full HD", "high").
			build(),

		Quality1080p6000k: newProfile(Quality1080p6000k, "1080p Medium (6 Mbps)", 1920, 1080, 6_000_000).
			withCodec("h264").
			withPreset("medium", 22).
			withNetwork(7.5).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withDescription("Balanced Full HD", "high").
			build(),

		Quality1080p8000k: newProfile(Quality1080p8000k, "1080p High (8 Mbps)", 1920, 1080, 8_000_000).
			withCodec("h264").
			withPreset("medium", 21).
			withNetwork(10.0).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withDescription("High-quality Full HD", "high").
			build(),

		Quality1080p12000k: newProfile(Quality1080p12000k, "1080p Ultra (12 Mbps)", 1920, 1080, 12_000_000).
			withCodec("h264", "h265").
			withPreset("slow", 20).
			withNetwork(15.0).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withDescription("Ultra Full HD quality", "ultra").
			build(),

		Quality1080p16000k: newProfile(Quality1080p16000k, "1080p Premium (16 Mbps)", 1920, 1080, 16_000_000).
			withCodec("h265", "vp9").
			withPreset("slow", 19).
			withNetwork(20.0).
			withScreen(1920, 1080).
			withDevices("tv").
			withDescription("Premium Full HD", "ultra").
			build(),

		// 1440p - 2K (4 variants)
		Quality1440p8000k: newProfile(Quality1440p8000k, "1440p Low (8 Mbps)", 2560, 1440, 8_000_000).
			withCodec("h265", "vp9").
			withPreset("medium", 22).
			withNetwork(10.0).
			withScreen(2560, 1440).
			withDevices("desktop", "tv").
			withDescription("Entry-level 2K", "ultra").
			build(),

		Quality1440p12000k: newProfile(Quality1440p12000k, "1440p Medium (12 Mbps)", 2560, 1440, 12_000_000).
			withCodec("h265", "vp9").
			withPreset("medium", 21).
			withNetwork(15.0).
			withScreen(2560, 1440).
			withDevices("desktop", "tv").
			withDescription("Balanced 2K quality", "ultra").
			build(),

		Quality1440p16000k: newProfile(Quality1440p16000k, "1440p High (16 Mbps)", 2560, 1440, 16_000_000).
			withCodec("h265", "vp9").
			withPreset("slow", 20).
			withNetwork(20.0).
			withScreen(2560, 1440).
			withDevices("tv").
			withDescription("High-quality 2K", "ultra").
			build(),

		Quality1440p24000k: newProfile(Quality1440p24000k, "1440p Ultra (24 Mbps)", 2560, 1440, 24_000_000).
			withCodec("h265", "vp9").
			withPreset("slow", 19).
			withNetwork(30.0).
			withScreen(2560, 1440).
			withDevices("tv").
			withDescription("Ultra 2K quality", "ultra").
			build(),

		// 4K - Ultra HD (7 variants)
		Quality4k16000k: newProfile(Quality4k16000k, "4K Low (16 Mbps)", 3840, 2160, 16_000_000).
			withCodec("h265", "vp9", "av1").
			withPreset("medium", 23).
			withNetwork(20.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Entry-level 4K", "ultra").
			build(),

		Quality4k25000k: newProfile(Quality4k25000k, "4K Medium (25 Mbps)", 3840, 2160, 25_000_000).
			withCodec("h265", "vp9", "av1").
			withPreset("medium", 22).
			withNetwork(30.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Balanced 4K quality", "ultra").
			build(),

		Quality4k35000k: newProfile(Quality4k35000k, "4K High (35 Mbps)", 3840, 2160, 35_000_000).
			withCodec("h265", "vp9", "av1").
			withPreset("slow", 21).
			withNetwork(40.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("High-quality 4K", "ultra").
			build(),

		Quality4k50000k: newProfile(Quality4k50000k, "4K Ultra (50 Mbps)", 3840, 2160, 50_000_000).
			withCodec("h265", "av1").
			withPreset("slow", 20).
			withNetwork(60.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Ultra 4K quality", "ultra").
			build(),

		Quality4k80000k: newProfile(Quality4k80000k, "4K Premium (80 Mbps)", 3840, 2160, 80_000_000).
			withCodec("h265", "av1").
			withPreset("veryslow", 18).
			withNetwork(100.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Premium 4K for maximum quality", "ultra").
			build(),

		Quality4k100000k: newProfile(Quality4k100000k, "4K Extreme (100 Mbps)", 3840, 2160, 100_000_000).
			withCodec("h265", "av1").
			withPreset("veryslow", 17).
			withNetwork(125.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Extreme 4K quality for Blu-ray rips", "ultra").
			build(),

		Quality4k120000k: newProfile(Quality4k120000k, "4K Reference (120 Mbps)", 3840, 2160, 120_000_000).
			withCodec("h265", "av1").
			withPreset("veryslow", 16).
			withNetwork(150.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withDescription("Reference quality 4K for premium Blu-ray direct play", "ultra").
			build(),

		// High Frame Rate (60 FPS) - 4 variants
		Quality720p8000k60fps: newProfile(Quality720p8000k60fps, "720p HFR (8 Mbps @ 60fps)", 1280, 720, 8_000_000).
			withCodec("h264", "h265", "vp9").
			withPreset("medium", 21).
			withNetwork(10.0).
			withScreen(1280, 720).
			withDevices("desktop", "tv").
			withFrameRate(60.0).
			withDescription("High frame rate 720p for smooth motion", "high").
			build(),

		Quality1080p12000k60fps: newProfile(Quality1080p12000k60fps, "1080p HFR (12 Mbps @ 60fps)", 1920, 1080, 12_000_000).
			withCodec("h264", "h265", "vp9").
			withPreset("medium", 20).
			withNetwork(15.0).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withFrameRate(60.0).
			withDescription("High frame rate 1080p for sports and action", "ultra").
			build(),

		Quality1080p20000k60fps: newProfile(Quality1080p20000k60fps, "1080p HFR High (20 Mbps @ 60fps)", 1920, 1080, 20_000_000).
			withCodec("h264", "h265", "vp9").
			withPreset("slow", 19).
			withNetwork(25.0).
			withScreen(1920, 1080).
			withDevices("desktop", "tv").
			withFrameRate(60.0).
			withDescription("Premium HFR 1080p for maximum motion clarity", "ultra").
			build(),

		Quality4k50000k60fps: newProfile(Quality4k50000k60fps, "4K HFR (50 Mbps @ 60fps)", 3840, 2160, 50_000_000).
			withCodec("h265", "vp9", "av1").
			withPreset("slow", 19).
			withNetwork(60.0).
			withScreen(3840, 2160).
			withDevices("tv").
			withFrameRate(60.0).
			withDescription("4K high frame rate for premium viewing", "ultra").
			build(),

		// 3D Profiles (Top-and-Bottom) - 3 variants
		Quality1080p12000k3d: newProfile(Quality1080p12000k3d, "1080p 3D TAB (12 Mbps)", 1920, 1080, 12_000_000).
			withCodec("h264", "h265").
			withPreset("medium", 21).
			withNetwork(15.0).
			withScreen(1920, 1080).
			withDevices("tv").
			with3D("tab").
			withDescription("3D top-and-bottom for stereoscopic content", "high").
			build(),

		Quality1080p16000k3d: newProfile(Quality1080p16000k3d, "1080p 3D TAB High (16 Mbps)", 1920, 1080, 16_000_000).
			withCodec("h264", "h265").
			withPreset("medium", 20).
			withNetwork(20.0).
			withScreen(1920, 1080).
			withDevices("tv").
			with3D("tab").
			withDescription("High quality 3D for premium stereoscopic viewing", "ultra").
			build(),

		Quality4k50000k3d: newProfile(Quality4k50000k3d, "4K 3D TAB (50 Mbps)", 3840, 2160, 50_000_000).
			withCodec("h265", "av1").
			withPreset("slow", 19).
			withNetwork(60.0).
			withScreen(3840, 2160).
			withDevices("tv").
			with3D("tab").
			withDescription("4K 3D top-and-bottom for premium stereoscopic", "ultra").
			build(),

		// Ultra-wide (2.39:1 Cinemascope) Profiles - 2 variants
		Quality4k25000kWide: newProfile(Quality4k25000kWide, "4K Wide (25 Mbps @ 2.39:1)", 3840, 1606, 25_000_000).
			withCodec("h265", "vp9", "av1").
			withPreset("medium", 21).
			withNetwork(30.0).
			withScreen(3840, 1606).
			withDevices("desktop", "tv").
			withAspectRatio("2.39:1").
			withDescription("Cinemascope ultra-wide 4K", "ultra").
			build(),

		Quality4k80000kWide: newProfile(Quality4k80000kWide, "4K Wide Ultra (80 Mbps @ 2.39:1)", 3840, 1606, 80_000_000).
			withCodec("h265", "av1").
			withPreset("slow", 18).
			withNetwork(100.0).
			withScreen(3840, 1606).
			withDevices("tv").
			withAspectRatio("2.39:1").
			withDescription("Premium ultra-wide 4K for cinematic films", "ultra").
			build(),
	}
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
// Supports multiple formats:
//   - New bitrate-specific format: "4k-60m", "1080p-12m", "720p-6m"
//   - Legacy format: "360p", "720p", "1080p", "4k"
//   - Internal IDs: "4k-60000k", "1080p-12000k"
//   - Special "original" quality (uses highest available 4K profile)
func GetAdaptiveProfileForQuality(quality string) (*AdaptiveProfile, error) {
	// Handle "original" quality - use the highest 4K profile as base
	// The session manager will adjust parameters based on source media
	if quality == "original" {
		// Use the highest 4K profile (120Mbps) as base for "original" quality
		// This ensures we get maximum quality transcoding
		return GetAdaptiveProfile(Quality4k120000k)
	}

	// First, try direct lookup (handles internal IDs like "4k-60000k")
	if profile, exists := adaptiveProfiles[quality]; exists {
		return profile, nil
	}

	// Try converting user-friendly format (e.g., "4k-60m") to internal ID (e.g., "4k-60000k")
	profileID := convertQualityToProfileID(quality)
	if profile, exists := adaptiveProfiles[profileID]; exists {
		return profile, nil
	}

	// Fall back to legacy quality mapping
	switch quality {
	case transcode.Quality360p:
		profileID = Quality360p800k
	case transcode.Quality480p:
		profileID = Quality480p1800k
	case transcode.Quality720p:
		profileID = Quality720p4000k // Use 4Mbps as default for "720p"
	case transcode.Quality1080p:
		profileID = Quality1080p8000k // Use 8Mbps as default for "1080p"
	case transcode.Quality4K:
		profileID = Quality4k25000k // Use 25Mbps as default for "4k"
	default:
		return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
	}
	return GetAdaptiveProfile(profileID)
}

// convertQualityToProfileID converts user-friendly quality names to internal profile IDs.
// e.g., "4k-60m" -> "4k-60000k", "1080p-12m" -> "1080p-12000k"
func convertQualityToProfileID(quality string) string {
	// Map of user-friendly suffixes to internal format
	bitrateMapping := map[string]string{
		// 480p variants
		"480p-2m": Quality480p1800k, // closest match

		// 720p variants
		"720p-3m": Quality720p2500k,
		"720p-6m": Quality720p5500k, // closest match

		// 1080p variants
		"1080p-6m":  Quality1080p6000k,
		"1080p-12m": Quality1080p12000k,
		"1080p-15m": Quality1080p16000k, // closest match
		"1080p-20m": Quality1080p16000k, // closest match

		// 4K variants
		"4k-20m":  Quality4k16000k, // closest match
		"4k-25m":  Quality4k25000k,
		"4k-35m":  Quality4k35000k,
		"4k-50m":  Quality4k50000k,
		"4k-60m":  Quality4k50000k, // closest match
		"4k-80m":  Quality4k80000k,
		"4k-100m": Quality4k100000k,
		"4k-120m": Quality4k120000k,
	}

	if profileID, exists := bitrateMapping[quality]; exists {
		return profileID
	}
	return quality
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
