// Package streaming provides domain types for adaptive streaming quality profiles.
package streaming

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
	AudioBitrate     int    // bits per second
	AudioChannels    int    // Target channels (2=stereo, 6=5.1, 8=7.1)
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

// QualityVariant represents a single quality variant in the quality ladder.
// This is the SINGLE SOURCE OF TRUTH for all quality levels.
// Both master playlist generation and transcoding profiles use this.
type QualityVariant struct {
	// ID is the unique identifier used in URLs and profile lookups (e.g., "4k-20m")
	ID string

	// Bandwidth in bits per second
	Bandwidth int

	// Resolution
	Width  int
	Height int

	// HLS codec string for EXT-X-STREAM-INF
	Codecs string
}

// ABRVariant is an alias for QualityVariant for backwards compatibility.
// Deprecated: Use QualityVariant instead.
type ABRVariant = QualityVariant

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

// Quality profile IDs - these match the QualityLadder IDs exactly
const (
	// Quality360p - Low quality fallback
	Quality360p = "360p"

	// Quality480p - SD quality
	Quality480p = "480p"

	// Quality720p2m - HD quality at 2 Mbps
	Quality720p2m = "720p-2m"
	// Quality720p4m - HD quality at 4 Mbps
	Quality720p4m = "720p-4m"

	// Quality1080p4m - Full HD at 4 Mbps
	Quality1080p4m = "1080p-4m"
	// Quality1080p10m - Full HD at 10 Mbps
	Quality1080p10m = "1080p-10m"
	// Quality1080p20m - Full HD at 20 Mbps
	Quality1080p20m = "1080p-20m"
	// Quality1080p40m - Full HD at 40 Mbps
	Quality1080p40m = "1080p-40m"
	// Quality1080p60m - Full HD at 60 Mbps
	Quality1080p60m = "1080p-60m"

	// Quality4k15m - 4K at 15 Mbps
	Quality4k15m = "4k-15m"
	// Quality4k20m - 4K at 20 Mbps
	Quality4k20m = "4k-20m"
	// Quality4k25m - 4K at 25 Mbps
	Quality4k25m = "4k-25m"
	// Quality4k40m - 4K at 40 Mbps
	Quality4k40m = "4k-40m"
	// Quality4k60m - 4K at 60 Mbps
	Quality4k60m = "4k-60m"
	// Quality4k100m - 4K at 100 Mbps
	Quality4k100m = "4k-100m"
	// Quality4k200m - 4K at 200 Mbps
	Quality4k200m = "4k-200m"
)

// Quality tiers
const (
	QualityTierLow    = "low"
	QualityTierMedium = "medium"
	QualityTierHigh   = "high"
	QualityTierUltra  = "ultra"
)

// Device types
const (
	DeviceTypeMobile  = "mobile"
	DeviceTypeTablet  = "tablet"
	DeviceTypeDesktop = "desktop"
	DeviceTypeTV      = "tv"
)
