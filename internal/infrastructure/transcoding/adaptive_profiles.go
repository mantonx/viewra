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
	AudioBitrate       int // bits per second
	AudioChannels      int // Target channels (2=stereo, 6=5.1, 8=7.1)
	AudioSampleRate    int
	PreserveMultiCh    bool // If true, keep original multi-channel audio (no downmix)
	AudioCodec         string // Target audio codec: "aac", "ac3", "eac3", "opus"
	MaxAudioChannels   int // Maximum channels to preserve (0 = no limit)

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
	FrameRate    float64 // Target frame rate (24, 30, 60)
	AspectRatio  string  // "16:9", "21:9", "2.39:1", etc.
	Is3D         bool    // Is this a 3D profile
	StereoMode   string  // "sbs" (side-by-side), "tab" (top-and-bottom), "" (2D)

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
	Quality720p8000k60fps  = "720p-8000k-60fps"
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

// adaptiveProfiles defines the complete catalog of granular quality profiles
var adaptiveProfiles = map[string]*AdaptiveProfile{
	// 240p - Ultra Low (Poor connections, data saving)
	Quality240p400k: {
		ID:               Quality240p400k,
		DisplayName:      "240p Ultra Low (0.4 Mbps)",
		Width:            426,
		Height:           240,
		VideoBitrate:     400_000,
		VideoMaxRate:     440_000,
		VideoBufSize:     800_000,
		AudioBitrate:     64_000,
		AudioChannels:    2,
		AudioSampleRate:  44100,
		PreserveMultiCh:  false, // Force stereo to save bandwidth
		AudioCodec:       "aac",
		MaxAudioChannels: 2,
		PreferredCodec:   "h264",
		FallbackCodecs:   []string{},
		Preset:          "fast",
		CRF:             28,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  0.5,
		MinScreenWidth:  320,
		MinScreenHeight: 240,
		RecommendedFor:  []string{"mobile"},
		DataUsageMBPerHour: 180,
		Description:     "Minimum quality for very poor connections",
		QualityTier:     "low",
	},

	// 360p - Low
	Quality360p800k: {
		ID:              Quality360p800k,
		DisplayName:     "360p Low (0.8 Mbps)",
		Width:           640,
		Height:          360,
		VideoBitrate:    800_000,
		VideoMaxRate:    880_000,
		VideoBufSize:    1_600_000,
		AudioBitrate:    96_000,
		AudioChannels:   2,
		AudioSampleRate: 44100,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{},
		Preset:          "fast",
		CRF:             26,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  1.0,
		MinScreenWidth:  480,
		MinScreenHeight: 320,
		RecommendedFor:  []string{"mobile"},
		DataUsageMBPerHour: 360,
		Description:     "Basic quality for mobile devices",
		QualityTier:     "low",
	},

	// 480p - Standard Definition (3 variants)
	Quality480p1200k: {
		ID:              Quality480p1200k,
		DisplayName:     "480p Low (1.2 Mbps)",
		Width:           854,
		Height:          480,
		VideoBitrate:    1_200_000,
		VideoMaxRate:    1_320_000,
		VideoBufSize:    2_400_000,
		AudioBitrate:    96_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{},
		Preset:          "medium",
		CRF:             24,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  1.5,
		MinScreenWidth:  640,
		MinScreenHeight: 480,
		RecommendedFor:  []string{"mobile", "tablet"},
		DataUsageMBPerHour: 540,
		Description:     "Entry-level SD quality",
		QualityTier:     "medium",
	},

	Quality480p1800k: {
		ID:              Quality480p1800k,
		DisplayName:     "480p Medium (1.8 Mbps)",
		Width:           854,
		Height:          480,
		VideoBitrate:    1_800_000,
		VideoMaxRate:    1_980_000,
		VideoBufSize:    3_600_000,
		AudioBitrate:    128_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{},
		Preset:          "medium",
		CRF:             23,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  2.2,
		MinScreenWidth:  640,
		MinScreenHeight: 480,
		RecommendedFor:  []string{"mobile", "tablet"},
		DataUsageMBPerHour: 810,
		Description:     "Balanced SD quality",
		QualityTier:     "medium",
	},

	Quality480p2500k: {
		ID:              Quality480p2500k,
		DisplayName:     "480p High (2.5 Mbps)",
		Width:           854,
		Height:          480,
		VideoBitrate:    2_500_000,
		VideoMaxRate:    2_750_000,
		VideoBufSize:    5_000_000,
		AudioBitrate:    128_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{},
		Preset:          "medium",
		CRF:             22,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  3.0,
		MinScreenWidth:  640,
		MinScreenHeight: 480,
		RecommendedFor:  []string{"tablet"},
		DataUsageMBPerHour: 1125,
		Description:     "High quality SD",
		QualityTier:     "medium",
	},

	// 720p - HD (4 variants)
	Quality720p2500k: {
		ID:              Quality720p2500k,
		DisplayName:     "720p Low (2.5 Mbps)",
		Width:           1280,
		Height:          720,
		VideoBitrate:    2_500_000,
		VideoMaxRate:    2_750_000,
		VideoBufSize:    5_000_000,
		AudioBitrate:    128_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265"},
		Preset:          "medium",
		CRF:             23,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  3.0,
		MinScreenWidth:  1024,
		MinScreenHeight: 576,
		RecommendedFor:  []string{"tablet", "desktop"},
		DataUsageMBPerHour: 1125,
		Description:     "Entry-level HD",
		QualityTier:     "high",
	},

	Quality720p4000k: {
		ID:              Quality720p4000k,
		DisplayName:     "720p Medium (4 Mbps)",
		Width:           1280,
		Height:          720,
		VideoBitrate:    4_000_000,
		VideoMaxRate:    4_400_000,
		VideoBufSize:    8_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             22,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  5.0,
		MinScreenWidth:  1024,
		MinScreenHeight: 576,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 1800,
		Description:     "Balanced HD quality",
		QualityTier:     "high",
	},

	Quality720p5500k: {
		ID:              Quality720p5500k,
		DisplayName:     "720p High (5.5 Mbps)",
		Width:           1280,
		Height:          720,
		VideoBitrate:    5_500_000,
		VideoMaxRate:    6_050_000,
		VideoBufSize:    11_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  7.0,
		MinScreenWidth:  1024,
		MinScreenHeight: 576,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 2475,
		Description:     "High quality HD",
		QualityTier:     "high",
	},

	Quality720p7500k: {
		ID:              Quality720p7500k,
		DisplayName:     "720p Ultra (7.5 Mbps)",
		Width:           1280,
		Height:          720,
		VideoBitrate:    7_500_000,
		VideoMaxRate:    8_250_000,
		VideoBufSize:    15_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9", "av1"},
		Preset:          "medium",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  9.0,
		MinScreenWidth:  1024,
		MinScreenHeight: 576,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 3375,
		Description:     "Premium HD quality",
		QualityTier:     "ultra",
	},

	// 1080p - Full HD (5 variants)
	Quality1080p4000k: {
		ID:              Quality1080p4000k,
		DisplayName:     "1080p Low (4 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    4_000_000,
		VideoMaxRate:    4_400_000,
		VideoBufSize:    8_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             23,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  5.5,
		MinScreenWidth:  1600,
		MinScreenHeight: 900,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 1800,
		Description:     "Entry-level Full HD",
		QualityTier:     "high",
	},

	Quality1080p6000k: {
		ID:              Quality1080p6000k,
		DisplayName:     "1080p Medium (6 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    6_000_000,
		VideoMaxRate:    6_600_000,
		VideoBufSize:    12_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             22,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  7.5,
		MinScreenWidth:  1600,
		MinScreenHeight: 900,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 2700,
		Description:     "Balanced Full HD",
		QualityTier:     "high",
	},

	Quality1080p8000k: {
		ID:              Quality1080p8000k,
		DisplayName:     "1080p High (8 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    8_000_000,
		VideoMaxRate:    8_800_000,
		VideoBufSize:    16_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9", "av1"},
		Preset:          "medium",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  10.0,
		MinScreenWidth:  1600,
		MinScreenHeight: 900,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 3600,
		Description:     "High quality Full HD",
		QualityTier:     "ultra",
	},

	Quality1080p12000k: {
		ID:              Quality1080p12000k,
		DisplayName:     "1080p Very High (12 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    12_000_000,
		VideoMaxRate:    13_200_000,
		VideoBufSize:    24_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9", "av1"},
		Preset:          "medium",
		CRF:             19,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  15.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 5400,
		Description:     "Very high quality Full HD",
		QualityTier:     "ultra",
	},

	Quality1080p16000k: {
		ID:              Quality1080p16000k,
		DisplayName:     "1080p Ultra (16 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    16_000_000,
		VideoMaxRate:    17_600_000,
		VideoBufSize:    32_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9", "av1"},
		Preset:          "slow",
		CRF:             18,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  20.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 7200,
		Description:     "Premium Full HD for best quality",
		QualityTier:     "ultra",
	},

	// 1440p - 2K (4 variants)
	Quality1440p8000k: {
		ID:              Quality1440p8000k,
		DisplayName:     "1440p Low (8 Mbps)",
		Width:           2560,
		Height:          1440,
		VideoBitrate:    8_000_000,
		VideoMaxRate:    8_800_000,
		VideoBufSize:    16_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1", "h264"},
		Preset:          "medium",
		CRF:             23,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  10.0,
		MinScreenWidth:  2048,
		MinScreenHeight: 1152,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 3600,
		Description:     "Entry-level 2K",
		QualityTier:     "ultra",
	},

	Quality1440p12000k: {
		ID:              Quality1440p12000k,
		DisplayName:     "1440p Medium (12 Mbps)",
		Width:           2560,
		Height:          1440,
		VideoBitrate:    12_000_000,
		VideoMaxRate:    13_200_000,
		VideoBufSize:    24_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1", "h264"},
		Preset:          "medium",
		CRF:             22,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  15.0,
		MinScreenWidth:  2048,
		MinScreenHeight: 1152,
		RecommendedFor:  []string{"desktop"},
		DataUsageMBPerHour: 5400,
		Description:     "Balanced 2K quality",
		QualityTier:     "ultra",
	},

	Quality1440p16000k: {
		ID:              Quality1440p16000k,
		DisplayName:     "1440p High (16 Mbps)",
		Width:           2560,
		Height:          1440,
		VideoBitrate:    16_000_000,
		VideoMaxRate:    17_600_000,
		VideoBufSize:    32_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1", "h264"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  20.0,
		MinScreenWidth:  2560,
		MinScreenHeight: 1440,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 7200,
		Description:     "High quality 2K",
		QualityTier:     "ultra",
	},

	Quality1440p24000k: {
		ID:              Quality1440p24000k,
		DisplayName:     "1440p Ultra (24 Mbps)",
		Width:           2560,
		Height:          1440,
		VideoBitrate:    24_000_000,
		VideoMaxRate:    26_400_000,
		VideoBufSize:    48_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "slow",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  30.0,
		MinScreenWidth:  2560,
		MinScreenHeight: 1440,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 10800,
		Description:     "Premium 2K quality",
		QualityTier:     "ultra",
	},

	// 4K - Ultra HD (5 variants)
	Quality4k16000k: {
		ID:              Quality4k16000k,
		DisplayName:     "4K Low (16 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    16_000_000,
		VideoMaxRate:    17_600_000,
		VideoBufSize:    32_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "medium",
		CRF:             24,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  20.0,
		MinScreenWidth:  3200,
		MinScreenHeight: 1800,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 7200,
		Description:     "Entry-level 4K",
		QualityTier:     "ultra",
	},

	Quality4k25000k: {
		ID:              Quality4k25000k,
		DisplayName:     "4K Medium (25 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    25_000_000,
		VideoMaxRate:    27_500_000,
		VideoBufSize:    50_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  30.0,
		MinScreenWidth:  3200,
		MinScreenHeight: 1800,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 11250,
		Description:     "Balanced 4K quality",
		QualityTier:     "ultra",
	},

	Quality4k35000k: {
		ID:              Quality4k35000k,
		DisplayName:     "4K High (35 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    35_000_000,
		VideoMaxRate:    38_500_000,
		VideoBufSize:    70_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "medium",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  40.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 15750,
		Description:     "High quality 4K",
		QualityTier:     "ultra",
	},

	Quality4k50000k: {
		ID:              Quality4k50000k,
		DisplayName:     "4K Very High (50 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    50_000_000,
		VideoMaxRate:    55_000_000,
		VideoBufSize:    100_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "slow",
		CRF:             19,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  60.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 22500,
		Description:     "Very high quality 4K",
		QualityTier:     "ultra",
	},

	Quality4k80000k: {
		ID:              Quality4k80000k,
		DisplayName:     "4K Ultra (80 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    80_000_000,
		VideoMaxRate:    88_000_000,
		VideoBufSize:    160_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"av1"},
		Preset:          "slow",
		CRF:             18,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  100.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 36000,
		Description:     "Premium 4K for maximum quality",
		QualityTier:     "ultra",
	},

	Quality4k100000k: {
		ID:              Quality4k100000k,
		DisplayName:     "4K Extreme (100 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    100_000_000,
		VideoMaxRate:    110_000_000,
		VideoBufSize:    200_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"av1"},
		Preset:          "veryslow",
		CRF:             17,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		MinNetworkMbps:  125.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 45000,
		Description:     "Extreme 4K quality for Blu-ray rips",
		QualityTier:     "ultra",
	},

	Quality4k120000k: {
		ID:              Quality4k120000k,
		DisplayName:     "4K Reference (120 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    120_000_000,
		VideoMaxRate:    132_000_000,
		VideoBufSize:    240_000_000,
		AudioBitrate:    320_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"av1"},
		Preset:          "veryslow",
		CRF:             16,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       30.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  150.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 54000,
		Description:     "Reference quality 4K for premium Blu-ray direct play",
		QualityTier:     "ultra",
	},

	// High Frame Rate (60 FPS) Profiles
	Quality720p8000k60fps: {
		ID:              Quality720p8000k60fps,
		DisplayName:     "720p HFR (8 Mbps @ 60fps)",
		Width:           1280,
		Height:          720,
		VideoBitrate:    8_000_000,
		VideoMaxRate:    8_800_000,
		VideoBufSize:    16_000_000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         120, // 2 seconds at 60fps
		FrameRate:       60.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  10.0,
		MinScreenWidth:  1280,
		MinScreenHeight: 720,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 3600,
		Description:     "High frame rate 720p for smooth motion",
		QualityTier:     "high",
	},

	Quality1080p12000k60fps: {
		ID:              Quality1080p12000k60fps,
		DisplayName:     "1080p HFR (12 Mbps @ 60fps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    12_000_000,
		VideoMaxRate:    13_200_000,
		VideoBufSize:    24_000_000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "medium",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         120,
		FrameRate:       60.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  15.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 5400,
		Description:     "High frame rate 1080p for sports and action",
		QualityTier:     "ultra",
	},

	Quality1080p20000k60fps: {
		ID:              Quality1080p20000k60fps,
		DisplayName:     "1080p HFR High (20 Mbps @ 60fps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    20_000_000,
		VideoMaxRate:    22_000_000,
		VideoBufSize:    40_000_000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265", "vp9"},
		Preset:          "slow",
		CRF:             19,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         120,
		FrameRate:       60.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  25.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 9000,
		Description:     "Premium HFR 1080p for maximum motion clarity",
		QualityTier:     "ultra",
	},

	Quality4k50000k60fps: {
		ID:              Quality4k50000k60fps,
		DisplayName:     "4K HFR (50 Mbps @ 60fps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    50_000_000,
		VideoMaxRate:    55_000_000,
		VideoBufSize:    100_000_000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "slow",
		CRF:             19,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         120,
		FrameRate:       60.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  60.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 22500,
		Description:     "4K high frame rate for premium viewing",
		QualityTier:     "ultra",
	},

	// 3D Profiles (Top-and-Bottom)
	Quality1080p12000k3d: {
		ID:              Quality1080p12000k3d,
		DisplayName:     "1080p 3D TAB (12 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    12_000_000,
		VideoMaxRate:    13_200_000,
		VideoBufSize:    24_000_000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "16:9",
		Is3D:            true,
		StereoMode:      "tab",
		MinNetworkMbps:  15.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 5400,
		Description:     "3D top-and-bottom for stereoscopic content",
		QualityTier:     "high",
	},

	Quality1080p16000k3d: {
		ID:              Quality1080p16000k3d,
		DisplayName:     "1080p 3D TAB High (16 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    16_000_000,
		VideoMaxRate:    17_600_000,
		VideoBufSize:    32_000_000,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h265"},
		Preset:          "medium",
		CRF:             20,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "16:9",
		Is3D:            true,
		StereoMode:      "tab",
		MinNetworkMbps:  20.0,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 7200,
		Description:     "High quality 3D for premium stereoscopic viewing",
		QualityTier:     "ultra",
	},

	Quality4k50000k3d: {
		ID:              Quality4k50000k3d,
		DisplayName:     "4K 3D TAB (50 Mbps)",
		Width:           3840,
		Height:          2160,
		VideoBitrate:    50_000_000,
		VideoMaxRate:    55_000_000,
		VideoBufSize:    100_000_000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"av1"},
		Preset:          "slow",
		CRF:             19,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "16:9",
		Is3D:            true,
		StereoMode:      "tab",
		MinNetworkMbps:  60.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 2160,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 22500,
		Description:     "4K 3D top-and-bottom for premium stereoscopic",
		QualityTier:     "ultra",
	},

	// Ultra-wide (2.39:1 Cinemascope) Profiles
	Quality4k25000kWide: {
		ID:              Quality4k25000kWide,
		DisplayName:     "4K Wide (25 Mbps @ 2.39:1)",
		Width:           3840,
		Height:          1606, // 2.39:1 aspect ratio
		VideoBitrate:    25_000_000,
		VideoMaxRate:    27_500_000,
		VideoBufSize:    50_000_000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"vp9", "av1"},
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "2.39:1",
		MinNetworkMbps:  30.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 1606,
		RecommendedFor:  []string{"desktop", "tv"},
		DataUsageMBPerHour: 11250,
		Description:     "Cinemascope ultra-wide 4K",
		QualityTier:     "ultra",
	},

	Quality4k80000kWide: {
		ID:              Quality4k80000kWide,
		DisplayName:     "4K Wide Ultra (80 Mbps @ 2.39:1)",
		Width:           3840,
		Height:          1606,
		VideoBitrate:    80_000_000,
		VideoMaxRate:    88_000_000,
		VideoBufSize:    160_000_000,
		PreferredCodec:  "h265",
		FallbackCodecs:  []string{"av1"},
		Preset:          "slow",
		CRF:             18,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "2.39:1",
		MinNetworkMbps:  100.0,
		MinScreenWidth:  3840,
		MinScreenHeight: 1606,
		RecommendedFor:  []string{"tv"},
		DataUsageMBPerHour: 36000,
		Description:     "Premium ultra-wide 4K for cinematic films",
		QualityTier:     "ultra",
	},
}

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
		profile.AudioCodec = "eac3" // Dolby Digital Plus, supports Atmos
		profile.MaxAudioChannels = 0 // No limit, preserve all channels
	}
}

// init applies audio settings to all profiles on package initialization
func init() {
	for _, profile := range adaptiveProfiles {
		applyAudioSettings(profile)
	}
}
