// Package profile provides quality profile definitions and recommendations for adaptive streaming.
//
// The profile package is organized as follows:
//   - types.go: Core type definitions (this file)
//   - adaptive.go: AdaptiveProfile struct, ABR ladder, profile building
//   - recommender.go: QualityRecommender for client-based quality selection
package profile

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

// ABRVariant represents a single quality variant in the ABR ladder.
// This is the SINGLE SOURCE OF TRUTH for all quality levels.
// Both master playlist generation and transcoding profiles use this.
type ABRVariant struct {
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
