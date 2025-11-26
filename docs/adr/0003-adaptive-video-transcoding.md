# ADR 0003: Adaptive Video Transcoding System

## Status

**Accepted** - 2025-11-24
**Implemented** - 2025-11-26

## Context

### Current State

ViewRA currently implements a functional video streaming system with:

- **HLS streaming** with on-demand FFmpeg segment generation (DASH support deprecated)
- **Fixed quality profiles**: 360p, 720p, 1080p, 4K
- **Manual quality selection** via dropdown in video player
- **Progressive transcoding** starting from user-selected timestamp
- **Session management** for active transcode jobs
- **Strategy detection** (Direct Play, Remux, Remux+Audio, Transcode)

**Legacy System Deprecation**: The original DASH-based transcoding system with 4 basic `QualityProfile` entries (profiles.go) will be fully deprecated in favor of the new adaptive HLS system with granular `AdaptiveProfile` entries. This consolidates the codebase around a single, modern streaming protocol.

### Problems Identified

1. **No Intelligent Quality Selection**
   - Users must manually choose quality without guidance
   - System doesn't consider device capabilities, network speed, or screen resolution
   - Leads to poor UX: buffering on slow connections, unnecessary transcoding on fast ones

2. **Limited Quality Options**
   - Only 4 quality tiers (360p, 720p, 1080p, 4K)
   - No 240p for very poor connections (mobile in rural areas)
   - No 480p standard mobile tier
   - No 1440p for 2K displays

3. **No Adaptive Streaming**
   - Quality remains fixed throughout playback
   - Can't respond to changing network conditions
   - No automatic quality degradation when buffering

4. **Suboptimal Initial Experience**
   - No way to detect optimal starting quality
   - Users on mobile may accidentally start 4K stream on cellular
   - Desktop users with 4K screens may not realize source quality limits

5. **Single Codec Support**
   - Only H.264 transcoding implemented
   - No support for modern codecs (H.265, VP9, AV1) that save 30-50% bandwidth
   - Can't leverage hardware acceleration on newer devices

6. **Device-Agnostic Approach**
   - Same quality recommendations for phone vs. desktop
   - No consideration for battery level, power mode, or metered connections
   - No screen resolution awareness

### Requirements for Future Device Support

The system must be flexible enough to support:
- **Smart TVs**: 4K/8K, hardware decoding, high bandwidth
- **Mobile Apps**: iOS/Android native players, varying network conditions
- **Tablets**: Medium screens, touch interfaces, battery constraints
- **Game Consoles**: Specific codec requirements, controller input
- **Chromecast/AirPlay**: Transcoding offload to server
- **Web Browser Variety**: Safari, Chrome, Firefox with different codec support

## Decision

We will implement a **comprehensive adaptive transcoding system** that intelligently selects optimal video quality based on:
1. Client device capabilities (screen size, CPU, memory, battery)
2. Network conditions (speed, connection type, stability)
3. Video source characteristics (resolution, codec, bitrate)
4. User preferences (data saving mode, quality preference)

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────┐      ┌──────────────────┐               │
│  │  Capability      │      │  Network         │               │
│  │  Detection       │      │  Monitor         │               │
│  │                  │      │                  │               │
│  │  • Screen size   │      │  • Speed tests   │               │
│  │  • CPU cores     │      │  • Buffer health │               │
│  │  • Memory        │      │  • Connection    │               │
│  │  • Battery       │      │    type          │               │
│  │  • Codec support │      │  • Latency       │               │
│  └────────┬─────────┘      └────────┬─────────┘               │
│           │                         │                          │
│           └─────────┬───────────────┘                          │
│                     ▼                                          │
│           ┌──────────────────────┐                            │
│           │  Quality Selector     │                            │
│           │                       │                            │
│           │  • Recommendation UI  │                            │
│           │  • Auto mode          │                            │
│           │  • Manual override    │                            │
│           │  • Adaptive switching │                            │
│           └──────────┬────────────┘                            │
│                      │                                          │
└──────────────────────┼──────────────────────────────────────────┘
                       │ HTTP Request
                       │ (capabilities + preferences)
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Server Layer                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│           ┌──────────────────────────┐                         │
│           │  Recommendation Engine   │                         │
│           │                          │                         │
│           │  • Screen filtering      │                         │
│           │  • Network assessment    │                         │
│           │  • Source analysis       │                         │
│           │  • Profile matching      │                         │
│           └──────────┬───────────────┘                         │
│                      │                                          │
│                      ▼                                          │
│           ┌──────────────────────────┐                         │
│           │  Adaptive Profiles       │                         │
│           │                          │                         │
│           │  240p, 360p, 480p,       │                         │
│           │  720p, 1080p, 1440p, 4K  │                         │
│           │                          │                         │
│           │  + Codec variants:       │                         │
│           │  H.264, H.265, VP9, AV1  │                         │
│           └──────────┬───────────────┘                         │
│                      │                                          │
│                      ▼                                          │
│           ┌──────────────────────────┐                         │
│           │  Transcode Engine        │                         │
│           │                          │                         │
│           │  • Session management    │                         │
│           │  • HLS generation        │                         │
│           │  • Hardware acceleration │                         │
│           │  • Multi-codec support   │                         │
│           └──────────────────────────┘                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. Client-Side Capability Detection

**Location**: `web/src/lib/capabilities/`

**Responsibilities**:
- Detect device type (mobile, tablet, desktop, TV)
- Measure network speed (Navigation Timing API + periodic tests)
- Identify screen resolution and pixel density
- Check codec support via Media Capabilities API
- Monitor battery status and power mode
- Track CPU cores and available memory

**Key Module**: `CapabilityDetector`
```typescript
interface ClientCapabilities {
  // Network
  networkSpeedMbps: number
  connectionType: 'wifi' | '4g' | '5g' | 'ethernet' | 'unknown'
  isMetered: boolean

  // Device
  deviceType: 'mobile' | 'tablet' | 'desktop' | 'tv'
  screenWidth: number
  screenHeight: number
  pixelRatio: number

  // Performance
  cpuCores: number
  memoryGB: number
  batteryLevel: number  // 0-1
  lowPowerMode: boolean

  // Media Support
  supportedCodecs: string[]
  hardwareAcceleration: boolean
}
```

#### 2. Quality Recommendation API

**Endpoint**: `POST /api/media/:id/recommend-quality`

**Request**:
```go
type QualityRecommendationRequest struct {
    // Network capabilities
    NetworkSpeed      float64 `json:"network_speed_mbps"`
    ConnectionType    string  `json:"connection_type"`
    IsMetered         bool    `json:"is_metered"`

    // Device capabilities
    DeviceType        string  `json:"device_type"`
    ScreenWidth       int     `json:"screen_width"`
    ScreenHeight      int     `json:"screen_height"`
    PixelRatio        float64 `json:"pixel_ratio"`

    // Performance
    CPUCores          int     `json:"cpu_cores"`
    MemoryGB          float64 `json:"memory_gb"`
    BatteryLevel      float64 `json:"battery_level"`
    LowPowerMode      bool    `json:"low_power_mode"`

    // Codec support
    SupportedCodecs   []string `json:"supported_codecs"`

    // User preferences
    PreferDataSaving  bool    `json:"prefer_data_saving"`
    PreferQuality     bool    `json:"prefer_quality"`
    ManualQuality     string  `json:"manual_quality,omitempty"`
}
```

**Response**:
```go
type QualityRecommendationResponse struct {
    RecommendedQuality string          `json:"recommended_quality"`
    AvailableQualities []string        `json:"available_qualities"`
    QualityOptions     []QualityOption `json:"quality_options"`
    Reasoning          string          `json:"reasoning"`
}

type QualityOption struct {
    Quality             string  `json:"quality"`
    Resolution          string  `json:"resolution"`
    EstimatedBitrate    string  `json:"estimated_bitrate"`
    RequiredNetworkMbps float64 `json:"required_network_mbps"`
    DataUsagePerHour    string  `json:"data_usage_per_hour"`
    IsRecommended       bool    `json:"is_recommended"`
    CanDirectPlay       bool    `json:"can_direct_play"`
    NeedsTranscode      bool    `json:"needs_transcode"`
    PreferredCodec      string  `json:"preferred_codec"`
}
```

#### 3. Granular Bitrate-Based Quality Profiles

**Location**: `internal/infrastructure/transcoding/profiles.go`

**Quality Naming Convention**: `{resolution}-{bitrate}` (e.g., "1080p-8000k", "4k-40000k")

**Why Granular Profiles?**

- Users get precise control over bandwidth vs quality tradeoff
- Better optimization for varying network conditions
- Matches how commercial platforms (Netflix, YouTube) present options
- Allows "Source" option for direct play at original bitrate

**Profile Structure**:

```go
type AdaptiveProfile struct {
    // Identity
    ID          string // "1080p-8000k"
    DisplayName string // "1080p High (8 Mbps)"

    // Resolution
    Width  int
    Height int

    // Bitrate (specific, not a range)
    VideoBitrate    int    // bits per second
    VideoMaxRate    int    // 110% of target for VBV
    VideoBufSize    int    // 2x target for VBV

    // Audio
    AudioBitrate    int
    AudioChannels   int
    AudioSampleRate int

    // Codec preferences
    PreferredCodec string   // "h264", "h265", "vp9", "av1"
    FallbackCodecs []string

    // Encoding parameters
    Preset          string // "ultrafast", "fast", "medium", "slow"
    CRF             int    // Constant Rate Factor (quality)
    EnableHWAccel   bool
    EnableFastStart bool

    // HLS segments
    SegmentDuration int
    GOPSize         int

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
```

**Complete Profile Catalog** (23 profiles total):

```go
var adaptiveProfiles = map[string]*AdaptiveProfile{
    // 240p - Ultra Low (Poor connections, data saving)
    "240p-400k": {
        ID:              "240p-400k",
        DisplayName:     "240p Ultra Low (0.4 Mbps)",
        Width:           426,
        Height:          240,
        VideoBitrate:    400_000,
        VideoMaxRate:    440_000,
        VideoBufSize:    800_000,
        AudioBitrate:    64_000,
        AudioChannels:   2,
        AudioSampleRate: 44100,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{},
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

    // 360p - Low (Mobile, limited bandwidth)
    "360p-800k": {
        ID:              "360p-800k",
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
        FallbackCodecs:  []string{"h265"},
        Preset:          "medium",
        CRF:             26,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  1.2,
        MinScreenWidth:  480,
        MinScreenHeight: 320,
        RecommendedFor:  []string{"mobile"},
        DataUsageMBPerHour: 360,
        Description:     "Basic quality for mobile devices",
        QualityTier:     "low",
    },

    // 480p - Standard Definition (3 variants)
    "480p-1200k": {
        ID:              "480p-1200k",
        DisplayName:     "480p Low (1.2 Mbps)",
        Width:           854,
        Height:          480,
        VideoBitrate:    1_200_000,
        VideoMaxRate:    1_320_000,
        VideoBufSize:    2_400_000,
        AudioBitrate:    128_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{"h265", "vp9"},
        Preset:          "medium",
        CRF:             24,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  2.0,
        MinScreenWidth:  640,
        MinScreenHeight: 360,
        RecommendedFor:  []string{"mobile", "tablet"},
        DataUsageMBPerHour: 540,
        Description:     "Standard definition for mobile",
        QualityTier:     "medium",
    },
    "480p-1800k": {
        ID:              "480p-1800k",
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
        FallbackCodecs:  []string{"h265", "vp9"},
        Preset:          "medium",
        CRF:             23,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  2.5,
        MinScreenWidth:  640,
        MinScreenHeight: 360,
        RecommendedFor:  []string{"mobile", "tablet"},
        DataUsageMBPerHour: 810,
        Description:     "Better SD quality",
        QualityTier:     "medium",
    },
    "480p-2500k": {
        ID:              "480p-2500k",
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
        FallbackCodecs:  []string{"h265", "vp9"},
        Preset:          "medium",
        CRF:             22,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  3.5,
        MinScreenWidth:  640,
        MinScreenHeight: 360,
        RecommendedFor:  []string{"tablet"},
        DataUsageMBPerHour: 1125,
        Description:     "High quality SD",
        QualityTier:     "medium",
    },

    // 720p - HD (4 variants)
    "720p-2500k": {
        ID:              "720p-2500k",
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
        FallbackCodecs:  []string{"h265", "vp9"},
        Preset:          "medium",
        CRF:             23,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  3.5,
        MinScreenWidth:  1024,
        MinScreenHeight: 576,
        RecommendedFor:  []string{"tablet", "desktop"},
        DataUsageMBPerHour: 1125,
        Description:     "HD ready for good connections",
        QualityTier:     "high",
    },
    "720p-4000k": {
        ID:              "720p-4000k",
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
        CRF:             21,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  5.5,
        MinScreenWidth:  1024,
        MinScreenHeight: 576,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 1800,
        Description:     "Balanced HD quality",
        QualityTier:     "high",
    },
    "720p-5500k": {
        ID:              "720p-5500k",
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
        CRF:             20,
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
    "720p-7500k": {
        ID:              "720p-7500k",
        DisplayName:     "720p Ultra (7.5 Mbps)",
        Width:           1280,
        Height:          720,
        VideoBitrate:    7_500_000,
        VideoMaxRate:    8_250_000,
        VideoBufSize:    15_000_000,
        AudioBitrate:    192_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{"h265", "vp9", "av1"},
        Preset:          "slow",
        CRF:             19,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  9.0,
        MinScreenWidth:  1024,
        MinScreenHeight: 576,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 3375,
        Description:     "Premium HD quality",
        QualityTier:     "high",
    },

    // 1080p - Full HD (5 variants)
    "1080p-4000k": {
        ID:              "1080p-4000k",
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
    "1080p-6000k": {
        ID:              "1080p-6000k",
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
        CRF:             21,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  8.0,
        MinScreenWidth:  1600,
        MinScreenHeight: 900,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 2700,
        Description:     "Balanced Full HD",
        QualityTier:     "high",
    },
    "1080p-8000k": {
        ID:              "1080p-8000k",
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
    "1080p-12000k": {
        ID:              "1080p-12000k",
        DisplayName:     "1080p Ultra (12 Mbps)",
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
        Preset:          "slow",
        CRF:             18,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  15.0,
        MinScreenWidth:  1600,
        MinScreenHeight: 900,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 5400,
        Description:     "Premium Full HD quality",
        QualityTier:     "ultra",
    },
    "1080p-16000k": {
        ID:              "1080p-16000k",
        DisplayName:     "1080p Max (16 Mbps)",
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
        CRF:             17,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  20.0,
        MinScreenWidth:  1600,
        MinScreenHeight: 900,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 7200,
        Description:     "Maximum Full HD quality",
        QualityTier:     "ultra",
    },

    // 1440p - 2K (4 variants)
    "1440p-8000k": {
        ID:              "1440p-8000k",
        DisplayName:     "1440p Low (8 Mbps)",
        Width:           2560,
        Height:          1440,
        VideoBitrate:    8_000_000,
        VideoMaxRate:    8_800_000,
        VideoBufSize:    16_000_000,
        AudioBitrate:    256_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{"h265", "vp9", "av1"},
        Preset:          "medium",
        CRF:             22,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  10.0,
        MinScreenWidth:  2048,
        MinScreenHeight: 1152,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 3600,
        Description:     "2K for high-res displays",
        QualityTier:     "ultra",
    },
    "1440p-12000k": {
        ID:              "1440p-12000k",
        DisplayName:     "1440p Medium (12 Mbps)",
        Width:           2560,
        Height:          1440,
        VideoBitrate:    12_000_000,
        VideoMaxRate:    13_200_000,
        VideoBufSize:    24_000_000,
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
        MinNetworkMbps:  15.0,
        MinScreenWidth:  2048,
        MinScreenHeight: 1152,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 5400,
        Description:     "Balanced 2K quality",
        QualityTier:     "ultra",
    },
    "1440p-16000k": {
        ID:              "1440p-16000k",
        DisplayName:     "1440p High (16 Mbps)",
        Width:           2560,
        Height:          1440,
        VideoBitrate:    16_000_000,
        VideoMaxRate:    17_600_000,
        VideoBufSize:    32_000_000,
        AudioBitrate:    320_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{"h265", "vp9", "av1"},
        Preset:          "slow",
        CRF:             19,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  20.0,
        MinScreenWidth:  2048,
        MinScreenHeight: 1152,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 7200,
        Description:     "High quality 2K",
        QualityTier:     "ultra",
    },
    "1440p-24000k": {
        ID:              "1440p-24000k",
        DisplayName:     "1440p Ultra (24 Mbps)",
        Width:           2560,
        Height:          1440,
        VideoBitrate:    24_000_000,
        VideoMaxRate:    26_400_000,
        VideoBufSize:    48_000_000,
        AudioBitrate:    320_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h264",
        FallbackCodecs:  []string{"h265", "vp9", "av1"},
        Preset:          "slow",
        CRF:             17,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  30.0,
        MinScreenWidth:  2048,
        MinScreenHeight: 1152,
        RecommendedFor:  []string{"desktop"},
        DataUsageMBPerHour: 10800,
        Description:     "Premium 2K quality",
        QualityTier:     "ultra",
    },

    // 4K - Ultra HD (5 variants)
    "4k-16000k": {
        ID:              "4k-16000k",
        DisplayName:     "4K Low (16 Mbps)",
        Width:           3840,
        Height:          2160,
        VideoBitrate:    16_000_000,
        VideoMaxRate:    17_600_000,
        VideoBufSize:    32_000_000,
        AudioBitrate:    320_000,
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
        MinNetworkMbps:  20.0,
        MinScreenWidth:  3200,
        MinScreenHeight: 1800,
        RecommendedFor:  []string{"desktop", "tv"},
        DataUsageMBPerHour: 7200,
        Description:     "Entry-level 4K (H.265 recommended)",
        QualityTier:     "ultra",
    },
    "4k-25000k": {
        ID:              "4k-25000k",
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
        FallbackCodecs:  []string{"vp9", "av1", "h264"},
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
    "4k-35000k": {
        ID:              "4k-35000k",
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
        FallbackCodecs:  []string{"vp9", "av1", "h264"},
        Preset:          "slow",
        CRF:             19,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  45.0,
        MinScreenWidth:  3200,
        MinScreenHeight: 1800,
        RecommendedFor:  []string{"desktop", "tv"},
        DataUsageMBPerHour: 15750,
        Description:     "High quality 4K",
        QualityTier:     "ultra",
    },
    "4k-50000k": {
        ID:              "4k-50000k",
        DisplayName:     "4K Ultra (50 Mbps)",
        Width:           3840,
        Height:          2160,
        VideoBitrate:    50_000_000,
        VideoMaxRate:    55_000_000,
        VideoBufSize:    100_000_000,
        AudioBitrate:    320_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h265",
        FallbackCodecs:  []string{"av1", "vp9", "h264"},
        Preset:          "slow",
        CRF:             17,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  65.0,
        MinScreenWidth:  3200,
        MinScreenHeight: 1800,
        RecommendedFor:  []string{"desktop", "tv"},
        DataUsageMBPerHour: 22500,
        Description:     "Premium 4K quality",
        QualityTier:     "ultra",
    },
    "4k-80000k": {
        ID:              "4k-80000k",
        DisplayName:     "4K Max (80 Mbps)",
        Width:           3840,
        Height:          2160,
        VideoBitrate:    80_000_000,
        VideoMaxRate:    88_000_000,
        VideoBufSize:    160_000_000,
        AudioBitrate:    320_000,
        AudioChannels:   2,
        AudioSampleRate: 48000,
        PreferredCodec:  "h265",
        FallbackCodecs:  []string{"av1", "vp9", "h264"},
        Preset:          "veryslow",
        CRF:             15,
        EnableHWAccel:   true,
        EnableFastStart: true,
        SegmentDuration: 2,
        GOPSize:         48,
        MinNetworkMbps:  100.0,
        MinScreenWidth:  3200,
        MinScreenHeight: 1800,
        RecommendedFor:  []string{"tv"},
        DataUsageMBPerHour: 36000,
        Description:     "Maximum 4K quality (near-lossless)",
        QualityTier:     "ultra",
    },
}
```

**Profile Selection Guidelines**:

| Network Speed | Recommended Profiles |
|---------------|---------------------|
| < 1 Mbps      | 240p-400k |
| 1-2 Mbps      | 360p-800k |
| 2-3 Mbps      | 480p-1200k |
| 3-5 Mbps      | 480p-2500k, 720p-2500k |
| 5-8 Mbps      | 720p-4000k, 1080p-4000k |
| 8-12 Mbps     | 720p-7500k, 1080p-6000k |
| 12-20 Mbps    | 1080p-8000k, 1080p-12000k, 1440p-8000k |
| 20-40 Mbps    | 1080p-16000k, 1440p-16000k, 4k-16000k |
| 40-65 Mbps    | 1440p-24000k, 4k-25000k, 4k-35000k |
| 65-100+ Mbps  | 4k-50000k, 4k-80000k |

**User Interface Example**:

```text
Quality Settings:
─────────────────────────────────────────────────────
⚡ Auto (Recommended)                    Currently: 1080p-8000k

Manual Selection:
  240p (0.4 Mbps)                       180 MB/hr    📱 Data Saver
  360p (0.8 Mbps)                       360 MB/hr    📱 Mobile
  ───────────────────────────────────────────────────────────────
  480p Low (1.2 Mbps)                   540 MB/hr    📱 Mobile
  480p Medium (1.8 Mbps)                810 MB/hr    📱 Tablet
  480p High (2.5 Mbps)                  1.1 GB/hr    📱 Tablet
  ───────────────────────────────────────────────────────────────
  720p Low (2.5 Mbps)                   1.1 GB/hr    💻 Desktop
  720p Medium (4 Mbps)         ✓        1.8 GB/hr    💻 Desktop
  720p High (5.5 Mbps)                  2.5 GB/hr    💻 Desktop
  720p Ultra (7.5 Mbps)                 3.4 GB/hr    💻 Desktop
  ───────────────────────────────────────────────────────────────
  1080p Low (4 Mbps)                    1.8 GB/hr    💻 Desktop
  1080p Medium (6 Mbps)                 2.7 GB/hr    💻 Desktop
  1080p High (8 Mbps)          👑       3.6 GB/hr    💻 Recommended
  1080p Ultra (12 Mbps)                 5.4 GB/hr    💻 Desktop
  1080p Max (16 Mbps)                   7.2 GB/hr    💻 Desktop
  ───────────────────────────────────────────────────────────────
  1440p Low (8 Mbps)                    3.6 GB/hr    🖥️  2K Display
  1440p Medium (12 Mbps)                5.4 GB/hr    🖥️  2K Display
  1440p High (16 Mbps)                  7.2 GB/hr    🖥️  2K Display
  1440p Ultra (24 Mbps)                 10.8 GB/hr   🖥️  2K Display
  ───────────────────────────────────────────────────────────────
  4K Low (16 Mbps)                      7.2 GB/hr    📺 4K Display
  4K Medium (25 Mbps)                   11.3 GB/hr   📺 4K Display
  4K High (35 Mbps)                     15.8 GB/hr   📺 4K Display
  4K Ultra (50 Mbps)                    22.5 GB/hr   📺 Premium
  4K Max (80 Mbps)                      36.0 GB/hr   📺 Max Quality
─────────────────────────────────────────────────────

Your network: 10.5 Mbps (WiFi) - Good connection
Your display: 1920x1080 - Full HD capable
Battery: 85% (charging)

💡 Tip: 1080p High offers the best quality for your setup
```

#### 4. Audio Compatibility & Direct Playback Priority

**Critical Design Principle**: Always prefer direct playback (no transcoding) when possible. Audio codec compatibility is a primary factor that determines if we can serve the original file directly.

**Browser Audio Codec Support**:

| Audio Codec | Chrome | Firefox | Safari | Edge | Mobile Support |
|-------------|--------|---------|--------|------|----------------|
| AAC         | ✅ Yes | ✅ Yes  | ✅ Yes | ✅ Yes | ✅ Universal |
| MP3         | ✅ Yes | ✅ Yes  | ✅ Yes | ✅ Yes | ✅ Universal |
| Opus        | ✅ Yes | ✅ Yes  | ⚠️ 11+ | ✅ Yes | ✅ Most |
| Vorbis      | ✅ Yes | ✅ Yes  | ❌ No  | ✅ Yes | ⚠️ Limited |
| AC3 (Dolby) | ❌ No  | ❌ No   | ❌ No  | ❌ No  | ❌ No |
| EAC3 (DD+)  | ❌ No  | ❌ No   | ❌ No  | ❌ No  | ❌ No |
| DTS         | ❌ No  | ❌ No   | ❌ No  | ❌ No  | ❌ No |
| TrueHD      | ❌ No  | ❌ No   | ❌ No  | ❌ No  | ❌ No |
| FLAC        | ✅ Yes | ✅ Yes  | ⚠️ 11+ | ✅ Yes | ⚠️ Limited |
| PCM         | ❌ No  | ❌ No   | ⚠️ Partial | ❌ No | ❌ No |

**Direct Playback Decision Matrix**:

```go
// Location: internal/infrastructure/transcoding/audio_compatibility.go

type AudioCompatibility struct {
    Codec           string
    Channels        int
    SampleRate      int
    Bitrate         int
    IsWebCompatible bool
    RequiresDownmix bool
    RequiresTranscode bool
}

// DetermineAudioCompatibility checks if audio can be played directly in browsers
func DetermineAudioCompatibility(audioInfo *AudioInfo) *AudioCompatibility {
    codec := strings.ToLower(audioInfo.Codec)
    channels := audioInfo.Channels

    compatibility := &AudioCompatibility{
        Codec:      codec,
        Channels:   channels,
        SampleRate: audioInfo.SampleRate,
        Bitrate:    audioInfo.Bitrate,
    }

    // Web-compatible codecs (can play directly)
    webCompatibleCodecs := map[string]bool{
        "aac":   true,
        "mp3":   true,
        "opus":  true,
        "mp4a":  true, // AAC variant
        "vorbis": true, // WebM containers
    }

    // Incompatible codecs (must transcode)
    incompatibleCodecs := map[string]bool{
        "ac3":     true, // Dolby Digital
        "eac3":    true, // Dolby Digital Plus
        "dts":     true, // DTS
        "dca":     true, // DTS alternate name
        "truehd":  true, // Dolby TrueHD
        "mlp":     true, // MLP (TrueHD)
        "pcm":     true, // Uncompressed PCM
        "pcm_s16le": true,
        "pcm_s24le": true,
        "flac":    true, // FLAC (limited browser support)
    }

    // Check codec compatibility
    if webCompatibleCodecs[codec] {
        compatibility.IsWebCompatible = true
    } else if incompatibleCodecs[codec] {
        compatibility.IsWebCompatible = false
        compatibility.RequiresTranscode = true
    } else {
        // Unknown codec - assume incompatible
        compatibility.IsWebCompatible = false
        compatibility.RequiresTranscode = true
    }

    // Check channel count - browsers typically support up to stereo natively
    // Multi-channel audio (5.1, 7.1) needs downmixing even if codec is compatible
    if channels > 2 {
        compatibility.RequiresDownmix = true
        // If codec is already incompatible, we'll transcode anyway
        // Otherwise, we can copy video and downmix audio only
        if compatibility.IsWebCompatible {
            compatibility.RequiresTranscode = false // Just downmix
        }
    }

    return compatibility
}

// CanDirectPlay determines if we can serve the file without any processing
func CanDirectPlay(videoInfo *VideoInfo, audioInfo *AudioInfo, containerFormat string) (bool, string) {
    // Check video codec
    isH264 := videoInfo.Codec == "h264" || videoInfo.Codec == "H264" || videoInfo.Codec == "avc1"
    if !isH264 {
        return false, fmt.Sprintf("Video codec %s not web-compatible (need H.264)", videoInfo.Codec)
    }

    // Check container format
    containerLower := strings.ToLower(containerFormat)
    isWebContainer := strings.Contains(containerLower, "mp4") ||
                      strings.Contains(containerLower, "webm") ||
                      (strings.Contains(containerLower, "mov") && !strings.Contains(containerLower, "quicktime"))

    if !isWebContainer {
        return false, fmt.Sprintf("Container format %s not web-compatible (need MP4/WebM)", containerFormat)
    }

    // Check audio compatibility
    audioCompat := DetermineAudioCompatibility(audioInfo)

    if !audioCompat.IsWebCompatible {
        return false, fmt.Sprintf("Audio codec %s not web-compatible (need AAC/MP3/Opus)", audioCompat.Codec)
    }

    if audioCompat.RequiresDownmix {
        return false, fmt.Sprintf("Audio has %d channels, browsers need stereo (2ch)", audioCompat.Channels)
    }

    // All checks passed - can direct play!
    return true, fmt.Sprintf("H.264 video + %s stereo audio in %s container", audioCompat.Codec, containerFormat)
}
```

**Transcoding Strategy Based on Audio**:

```go
// Enhanced DetermineStreamStrategy with audio awareness
func DetermineStreamStrategy(videoInfo *VideoInfo, audioInfo *AudioInfo) (StreamStrategy, string) {
    if videoInfo == nil || audioInfo == nil {
        return Transcode, "Missing media information"
    }

    // Check if we can direct play
    canDirectPlay, reason := CanDirectPlay(videoInfo, audioInfo, videoInfo.ContainerFormat)
    if canDirectPlay {
        return DirectPlay, reason
    }

    // Analyze what needs to be done
    isH264 := videoInfo.Codec == "h264" || videoInfo.Codec == "H264" || videoInfo.Codec == "avc1"
    audioCompat := DetermineAudioCompatibility(audioInfo)

    // Tier 1: Remux - Video is H.264, Audio is compatible, just wrong container
    // Copy both streams to HLS without re-encoding (2-5 minutes)
    if isH264 && audioCompat.IsWebCompatible && !audioCompat.RequiresDownmix {
        return Remux, fmt.Sprintf(
            "H.264 video + %s %dch audio compatible, only container needs remux from %s to HLS",
            audioCompat.Codec, audioCompat.Channels, videoInfo.ContainerFormat,
        )
    }

    // Tier 2: Remux with Audio Transcode - Video is H.264 but audio needs work
    // Copy video stream, transcode/downmix audio to AAC stereo (5-10 minutes)
    if isH264 {
        if audioCompat.RequiresDownmix {
            return RemuxWithAudioDownmix, fmt.Sprintf(
                "H.264 video compatible, but %s %d.%dch audio needs downmix to AAC stereo",
                audioCompat.Codec,
                audioCompat.Channels / 2, // Front channels
                audioCompat.Channels % 2, // LFE channel
            )
        }
        if !audioCompat.IsWebCompatible {
            return RemuxWithAudioDownmix, fmt.Sprintf(
                "H.264 video compatible, but %s audio needs transcode to AAC",
                audioCompat.Codec,
            )
        }
    }

    // Tier 3: Full Transcode - Video needs re-encoding (20-60 minutes)
    // Audio will be transcoded to AAC stereo as part of the process
    return Transcode, fmt.Sprintf(
        "Video codec %s incompatible, needs full transcode to H.264 (audio: %s %dch → AAC stereo)",
        videoInfo.Codec, audioCompat.Codec, audioCompat.Channels,
    )
}
```

**Audio Downmix Strategy**:

When downmixing multi-channel audio to stereo, use proper channel mapping to preserve clarity:

```go
// FFmpeg downmix filter for common channel layouts
func GetDownmixFilter(channels int) string {
    switch channels {
    case 6: // 5.1 (FL+FR+FC+LFE+BL+BR)
        // Downmix formula: Preserve dialog (FC), balance surrounds
        return "pan=stereo|FL=0.5*FL+0.707*FC+0.5*BL|FR=0.5*FR+0.707*FC+0.5*BR"

    case 8: // 7.1 (FL+FR+FC+LFE+BL+BR+SL+SR)
        // More complex downmix for 7.1
        return "pan=stereo|FL=0.4*FL+0.6*FC+0.4*BL+0.3*SL|FR=0.4*FR+0.6*FC+0.4*BR+0.3*SR"

    case 3: // 2.1 (FL+FR+LFE) or 3.0 (FL+FR+FC)
        // Simple center mix
        return "pan=stereo|FL=0.5*FL+0.707*FC|FR=0.5*FR+0.707*FC"

    default:
        // Generic downmix - let FFmpeg decide
        return "aresample=ocl=stereo"
    }
}
```

**Enhanced Profile Selection with Audio**:

The recommendation algorithm must now consider:

1. **Source Audio Analysis**:
   - Detect audio codec from source file
   - Determine if direct play is possible
   - Calculate additional bandwidth needed if audio transcoding required

2. **Quality Profile Adjustment**:
   - If audio needs transcoding, don't offer "Source" quality
   - Add bandwidth for AAC audio encoding (~128-320 kbps)
   - Adjust time estimates based on audio complexity

3. **User Communication**:
   - Show "⚡ Direct Play Available" when no transcoding needed
   - Show "🔊 Audio Transcode Required" when only audio needs work
   - Show "🎬 Full Transcode Required" when both video and audio need work

**Example Scenarios**:

| Source Video | Source Audio | Container | Result | Strategy | Reasoning |
|--------------|--------------|-----------|--------|----------|-----------|
| H.264 1080p | AAC Stereo | MP4 | ⚡ Direct Play | DirectPlay | Perfect match - instant playback |
| H.264 1080p | AAC Stereo | MKV | 📦 Container Remux | Remux | Video & audio compatible, only container needs change |
| H.264 720p | AAC 5.1 | MP4 | 🔊 Audio Downmix | RemuxWithAudioDownmix | Video good, audio needs stereo downmix |
| H.264 1080p | AC3 5.1 | MKV | 🔊 Audio Transcode | RemuxWithAudioDownmix | Video good, audio needs AAC transcode |
| H.264 1080p | DTS-HD 7.1 | MKV | 🔊 Audio Transcode | RemuxWithAudioDownmix | Video good, high-quality audio needs work |
| H.265 4K | TrueHD 7.1 | MKV | 🎬 Full Transcode | Transcode | Both video and audio incompatible |
| VP9 1080p | Opus Stereo | WebM | ⚡ Direct Play* | DirectPlay | WebM container with compatible codecs |
| MPEG-2 720p | AC3 5.1 | VOB | 🎬 Full Transcode | Transcode | DVD format - everything needs transcoding |

**UI Display Examples**:

```text
[Direct Play Available - No Transcoding]
─────────────────────────────────────────────────
⚡ Source Quality (Original)              Instant playback
   Resolution: 1920x1080 (Full HD)
   Video: H.264, 8 Mbps
   Audio: AAC Stereo, 192 kbps
   Container: MP4

   ✅ No processing required - plays instantly
─────────────────────────────────────────────────


[Container Remux Required]
─────────────────────────────────────────────────
📦 Source Quality (Container Remux)      ~2-5 min wait
   Resolution: 1920x1080 (Full HD)
   Video: H.264, 8 Mbps (copy, no re-encode)
   Audio: AAC Stereo, 192 kbps (copy, no re-encode)
   Container: MKV → HLS

   ℹ️  Only container format needs conversion - very fast

   Available now: 1080p-6000k ⚡
   (Lower quality, ready instantly)
─────────────────────────────────────────────────


[Audio Transcode Required]
─────────────────────────────────────────────────
🔊 Source Quality (Audio Processing)     ~5-10 min wait
   Resolution: 1920x1080 (Full HD)
   Video: H.264, 8 Mbps (copy, no re-encode)
   Audio: AC3 5.1 → AAC Stereo (transcode)
   Container: MKV → HLS

   ℹ️  Video will be copied (fast), audio transcoded

   Available now: 1080p-6000k ⚡
   (Lower quality, ready instantly)
─────────────────────────────────────────────────


[Full Transcode Required]
─────────────────────────────────────────────────
🎬 Source Quality (Full Processing)      ~20-60 min wait
   Resolution: 3840x2160 (4K)
   Video: H.265 → H.264 (re-encode)
   Audio: DTS-HD 7.1 → AAC Stereo (transcode)
   Container: MKV → HLS

   ⚠️  Both video and audio need processing

   Available now: 1080p-8000k ⚡
   (Playback while 4K transcodes in background)
─────────────────────────────────────────────────
```

**Recommendation Engine Integration**:

```go
func (uc *RecommendQualityUseCase) Execute(
    ctx context.Context,
    req QualityRecommendationRequest,
) (*QualityRecommendationResponse, error) {
    // ... existing capability detection ...

    // NEW: Analyze audio compatibility
    audioInfo, err := uc.videoInfoService.GetAudioInfo(mediaInfo.FilePath)
    if err != nil {
        // Log but continue - we'll assume audio needs transcoding
        uc.logger.Warn("failed to get audio info", "error", err)
    }

    // Check if direct play is possible
    canDirectPlay, directPlayReason := transcoding.CanDirectPlay(videoInfo, audioInfo, mediaInfo.Container)

    // Build response with audio information
    response := &QualityRecommendationResponse{
        RecommendedQuality: recommended.ID,
        AvailableQualities: qualities,
        QualityOptions:     options,
        Reasoning:          reasoning,
        SourceInfo: &SourceInfo{
            Width:      videoInfo.Width,
            Height:     videoInfo.Height,
            Codec:      videoInfo.Codec,
            Bitrate:    videoInfo.Bitrate,
            Duration:   int(videoInfo.Duration),

            // NEW: Audio information
            AudioCodec:    audioInfo.Codec,
            AudioChannels: audioInfo.Channels,
            AudioBitrate:  audioInfo.Bitrate,

            // NEW: Compatibility flags
            Compatible:        canDirectPlay,
            CompatibilityNote: directPlayReason,
            RequiresAudioTranscode: audioInfo != nil && !canDirectPlay && videoInfo.Codec == "h264",
            RequiresVideoTranscode: videoInfo.Codec != "h264",
        },
    }

    // If direct play available, add "Source" quality option
    if canDirectPlay {
        sourceOption := QualityOption{
            Quality:             "source",
            Resolution:          fmt.Sprintf("%dx%d", videoInfo.Width, videoInfo.Height),
            EstimatedBitrate:    fmt.Sprintf("%dk", videoInfo.Bitrate/1000),
            RequiredNetworkMbps: float64(videoInfo.Bitrate) / 1_000_000 * 1.2, // Add 20% headroom
            DataUsagePerHour:    fmt.Sprintf("%d MB", (videoInfo.Bitrate/8)*3600/1_000_000),
            IsRecommended:       false, // Only recommend if network can handle it
            CanDirectPlay:       true,
            NeedsTranscode:      false,
            PreferredCodec:      videoInfo.Codec,
            Description:         "Original quality - instant playback, no transcoding",
        }

        // Add as first option
        response.QualityOptions = append([]QualityOption{sourceOption}, response.QualityOptions...)
        response.AvailableQualities = append([]string{"source"}, response.AvailableQualities...)
    }

    return response, nil
}
```

This comprehensive audio compatibility system ensures we **always prioritize direct playback** when possible, while gracefully handling incompatible audio codecs through the most efficient transcoding strategy.

#### 5. Recommendation Algorithm

**Location**: `internal/application/transcode/recommend_quality.go`

**Algorithm Flow**:

```go
func RecommendQuality(req QualityRecommendationRequest, videoInfo *VideoInfo) (*QualityRecommendationResponse, error) {
    // 1. Get all available profiles
    profiles := GetAllProfiles()

    // 2. Filter by screen resolution (don't offer higher than display)
    effectiveHeight := req.ScreenHeight * int(req.PixelRatio)
    profiles = filterByScreenSize(profiles, effectiveHeight)

    // 3. Filter by network capability
    profiles = filterByNetworkSpeed(profiles, req.NetworkSpeed)

    // 4. Filter by source video (don't upscale)
    if videoInfo != nil {
        profiles = filterBySourceResolution(profiles, videoInfo)
    }

    // 5. Apply device-specific constraints
    if req.LowPowerMode || req.BatteryLevel < 0.2 {
        profiles = capForPowerSaving(profiles, "480p")
    }
    if req.IsMetered && !req.PreferQuality {
        profiles = capForDataSaving(profiles, "720p")
    }

    // 6. Apply user preferences
    if req.PreferDataSaving {
        profiles = biasTowardLower(profiles)
    } else if req.PreferQuality {
        profiles = biasTowardHigher(profiles)
    }

    // 7. Check for direct play opportunity
    if canDirectPlay(videoInfo, req.SupportedCodecs) {
        return recommendDirectPlay(videoInfo)
    }

    // 8. Select optimal profile
    recommended := selectOptimalProfile(profiles, req, videoInfo)

    // 9. Build response with all options
    return buildResponse(recommended, profiles, videoInfo)
}
```

**Recommendation Rules**:

1. **Screen-based filtering**:
   - Never offer qualities higher than screen resolution
   - Account for pixel ratio (Retina displays = 2x)
   - Example: 720p screen → max 720p offer

2. **Network-based filtering**:
   - 240p: < 1 Mbps (very poor connections)
   - 360p: 1-2 Mbps (poor connections)
   - 480p: 2-3 Mbps (standard mobile)
   - 720p: 3-5 Mbps (good connections)
   - 1080p: 5-8 Mbps (fast connections)
   - 1440p: 8-12 Mbps (very fast)
   - 4K: 12+ Mbps (premium broadband)

3. **Device-based adjustments**:
   - Mobile + battery < 20% → Cap at 720p max
   - Low power mode enabled → Cap at 480p max
   - Metered connection → Suggest data-saving, cap at 720p
   - Mobile device → Bias toward 480p-720p range
   - Desktop → Bias toward 720p-1080p range
   - TV → Bias toward 1080p-4K range

4. **Source-aware optimization**:
   - Never upscale (480p source → max 480p offer)
   - If source is H.264 + compatible audio → Prioritize direct play
   - If source is 720p → Don't offer 1080p/4K
   - If source bitrate < target → Skip that profile

5. **User preference weighting**:
   - Data saving mode → Reduce recommended quality by 1-2 tiers
   - Quality preference → Increase recommended quality by 1 tier
   - Manual override → Always honor user's explicit choice

#### 5. Enhanced Video Player Controls

**Location**: `web/src/components/media/VideoPlayer/VideoControls.tsx`

**Current State**: Lines 362-380 show basic quality dropdown

**Enhanced Quality Selector**:
```tsx
<QualitySelector
  currentQuality={currentQuality}
  recommendedQuality={recommendedQuality}
  availableQualities={qualityOptions}
  networkSpeed={currentNetworkSpeed}
  autoMode={isAutoMode}
  onQualityChange={handleQualityChange}
  onAutoToggle={handleAutoToggle}
/>
```

**New Features**:
- **Visual indicators**:
  - ✓ Green checkmark for recommended quality
  - ⚡ Lightning bolt for direct play (no transcoding needed)
  - 📶 Network strength indicator per quality
  - 💾 Estimated data usage per hour
  - 🔋 Battery impact indicator (mobile only)

- **Smart Auto mode**:
  - Starts at recommended quality
  - Monitors buffer health and network speed
  - Automatically switches qualities without disruption
  - Displays current selection: "Auto (Currently 720p)"
  - Smooth transitions at segment boundaries

- **User education**:
  - Tooltips explaining each quality
  - Data usage estimates
  - Quality-to-resolution mapping
  - "Why this recommendation?" explanation

#### 6. Real-Time Network Monitoring

**Location**: `web/src/lib/network/NetworkMonitor.ts`

**Responsibilities**:
- Continuous network speed measurement
- Buffer health monitoring via HLS.js events
- Detect network degradation/improvement
- Trigger quality adjustments when needed

**Adaptive Logic**:
```typescript
class NetworkMonitor {
  private currentSpeed: number
  private bufferHealth: number  // 0-1, ratio of buffered vs ideal

  // HLS.js integration
  watchBufferHealth(hls: Hls): void {
    hls.on(Hls.Events.FRAG_BUFFERED, (event, data) => {
      const buffered = data.frag.buffered
      const current = video.currentTime
      const bufferAhead = buffered - current

      this.bufferHealth = bufferAhead / IDEAL_BUFFER_SECONDS

      // Low buffer: consider downgrade
      if (this.bufferHealth < 0.3 && this.currentSpeed < requiredSpeed) {
        this.suggestQualityChange('down', 'buffering')
      }

      // High buffer + fast network: consider upgrade
      if (this.bufferHealth > 0.8 && this.currentSpeed > higherQualitySpeed) {
        this.suggestQualityChange('up', 'stable-network')
      }
    })
  }

  // Periodic speed tests
  async measureSpeed(): Promise<number> {
    const testSize = 500_000 // 500KB test chunk
    const start = performance.now()

    const response = await fetch('/api/speedtest/chunk', {
      headers: { 'Cache-Control': 'no-cache' }
    })
    const blob = await response.blob()

    const durationSeconds = (performance.now() - start) / 1000
    const speedMbps = (blob.size * 8) / durationSeconds / 1_000_000

    return speedMbps
  }
}
```

#### 7. Multi-Codec Support (Future)

**Phase 2 Enhancement**:

Support for modern codecs that provide better compression:

| Codec | Compression vs H.264 | Browser Support | Encode Speed | Use Case |
|-------|---------------------|-----------------|--------------|----------|
| H.264 | Baseline (100%)     | Universal       | Fast         | Fallback |
| H.265 | ~40% smaller        | Safari, Edge    | Medium       | Apple devices |
| VP9   | ~35% smaller        | Chrome, Firefox | Slow         | Google ecosystem |
| AV1   | ~50% smaller        | Chrome 90+      | Very slow    | Future premium |

**Implementation**:
```go
type CodecProfile struct {
    Codec            string
    HWAccelMethods   []string // ["vaapi", "nvenc", "qsv", "videotoolbox"]
    BrowserSupport   map[string]bool
    CompressionRatio float64  // vs H.264
    EncodeSpeed      float64  // relative multiplier
}

// Client sends supported codecs in Accept header:
// Accept: video/mp4; codecs="avc1.64001f, hev1.1.6.L93.B0, av01.0.05M.08"
```

**Codec Selection Priority**:
1. Check client's `Accept` header or capability report
2. If AV1 supported → Use for 1080p+, saves ~50% bandwidth
3. If H.265 supported → Use for 720p+, saves ~40% bandwidth
4. If VP9 supported → Use as alternative, saves ~35% bandwidth
5. Fallback to H.264 for universal compatibility

### Database Schema Changes

```sql
-- Extend transcode_jobs for adaptive features
ALTER TABLE transcode_jobs
    ADD COLUMN client_device_type VARCHAR(20),
    ADD COLUMN client_network_type VARCHAR(20),
    ADD COLUMN recommended_quality VARCHAR(10),
    ADD COLUMN codec VARCHAR(10) DEFAULT 'h264';

-- Track quality switches during playback
CREATE TABLE quality_switches (
    id BIGSERIAL PRIMARY KEY,
    media_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    session_id VARCHAR(50),
    from_quality VARCHAR(10),
    to_quality VARCHAR(10),
    reason VARCHAR(50), -- 'user', 'buffer_low', 'network_improved', 'auto'
    network_speed_mbps FLOAT,
    buffer_health FLOAT,
    timestamp TIMESTAMP DEFAULT NOW(),

    INDEX idx_media_quality (media_id, from_quality, to_quality),
    INDEX idx_session (session_id),
    INDEX idx_timestamp (timestamp)
);

-- Analytics for recommendation tuning
CREATE TABLE playback_metrics (
    id BIGSERIAL PRIMARY KEY,
    media_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    session_id VARCHAR(50),

    -- Playback details
    quality VARCHAR(10),
    codec VARCHAR(10),
    avg_bitrate INT,

    -- Quality of experience
    buffer_events INT,
    quality_switches INT,
    stall_duration_seconds INT,
    completion_rate FLOAT, -- 0-1

    -- Client context
    device_type VARCHAR(20),
    network_type VARCHAR(20),
    avg_network_speed_mbps FLOAT,

    -- Session timing
    session_duration_seconds INT,
    created_at TIMESTAMP DEFAULT NOW(),

    INDEX idx_media_device (media_id, device_type),
    INDEX idx_quality_metrics (quality, completion_rate),
    INDEX idx_timestamp (created_at)
);

-- User preferences for quality
CREATE TABLE user_video_preferences (
    user_id BIGINT PRIMARY KEY,

    -- Quality preferences
    quality_preference VARCHAR(10), -- 'auto', or specific quality
    prefer_data_saving BOOLEAN DEFAULT false,
    prefer_quality BOOLEAN DEFAULT false,

    -- Connection-specific overrides
    allow_cellular_hd BOOLEAN DEFAULT false,
    allow_cellular_4k BOOLEAN DEFAULT false,

    -- Codec preferences
    preferred_codec VARCHAR(10), -- NULL = auto

    updated_at TIMESTAMP DEFAULT NOW()
);
```

### API Endpoints

#### New Endpoints

1. **Quality Recommendation**
   - `POST /api/media/:id/recommend-quality`
   - Body: ClientCapabilities + UserPreferences
   - Returns: Recommended quality + all options with metadata

2. **Network Speed Test**
   - `GET /api/speedtest/chunk`
   - Returns: 500KB test chunk for speed measurement
   - Cached for 30 seconds to prevent abuse

3. **Update User Preferences**
   - `PUT /api/users/:id/video-preferences`
   - Body: VideoPreferences
   - Returns: Updated preferences

4. **Quality Switch Event**
   - `POST /api/playback/quality-switch`
   - Body: { mediaId, fromQuality, toQuality, reason, sessionId }
   - Returns: Acknowledgment
   - Used for analytics

#### Enhanced Existing Endpoints

1. **Playlist Endpoint** (`/api/media/:id/hls/:quality/playlist.m3u8`)
   - Supports `?start=X` query parameter for seeking to specific timestamp
   - Quality profile ID (e.g., `1080p-8000k`) implicitly specifies the codec
   - Session tracking handled internally via (mediaID, quality) tuple

2. **Media Info Endpoint**
   - Include available codecs for the source file
   - Include transcode status for all qualities/codecs

### Implementation Phases

#### **Phase 1: Foundation** (Week 1-2)
**Goal**: Basic capability detection and recommendation system

**Tasks**:
1. Create capability detection module (frontend)
   - ✅ Device detection
   - ✅ Network speed measurement
   - ✅ Screen resolution detection
   - ✅ Basic codec support detection

2. Add new quality profiles (backend)
   - ✅ 240p, 480p, 1440p profiles
   - ✅ Extended profile structure with ranges
   - ✅ Profile validation

3. Implement recommendation endpoint
   - ✅ Request/response types
   - ✅ Recommendation algorithm
   - ✅ Screen-based filtering
   - ✅ Network-based filtering
   - ✅ Source-aware filtering

4. Database schema updates
   - ✅ Migration for new columns
   - ✅ Create new tables (quality_switches, playback_metrics, user_video_preferences)

5. Basic UI enhancement
   - ✅ Show recommended quality with indicator (star ★ + tooltip)
   - ✅ Display all quality options with metadata
   - ✅ Add data usage estimates

6. **HLS Master Playlist Implementation** (backend)
   - ✅ Add master playlist endpoint: `GET /api/media/:id/hls/master.m3u8`
   - ✅ Generate HLS master playlist with all quality variants
   - ✅ Include bandwidth and resolution metadata for each variant
   - ✅ Support `?start=` parameter for resume-from-timestamp
   - ✅ Update frontend to use master playlist URL

**Deliverables**:
- ✅ Client can detect its own capabilities
- ✅ Server recommends optimal quality
- ✅ UI shows recommendation with reasoning (star indicator + hover tooltip)
- ✅ Fuzzy quality matching when exact height unavailable
- ✅ Master playlist enables multi-quality streaming
- ✅ Database tracks recommendation accuracy

**Success Metrics**:
- Recommendation accuracy > 80% (users don't override)
- Quality selection time < 2 seconds
- No regressions in existing playback

---

#### **Phase 2: Adaptive Streaming** (Week 3-4)
**Goal**: Real-time quality adaptation during playback

**Tasks**:
1. Network monitoring service (frontend)
   - Continuous speed measurement
   - Buffer health tracking via HLS.js
   - Network degradation detection

2. Auto quality mode implementation
   - Automatic quality switching logic
   - Smooth transitions at segment boundaries
   - User notification of quality changes

3. Quality switch endpoint
   - Analytics collection
   - Session tracking
   - Reason categorization

4. Enhanced player controls
   - Auto mode toggle
   - Current quality indicator
   - Quality switch notifications
   - Network speed indicator

5. User preference persistence
   - LocalStorage for client-side preferences
   - API for server-side preferences
   - Preference sync across devices

**Deliverables**:
- ✅ Auto mode dynamically adjusts quality
- ✅ Users see quality changes with reasons
- ✅ System learns from user overrides
- ✅ Preferences persist across sessions

**Success Metrics**:
- Buffer rate < 1% of playback time
- Quality switches < 3 per session
- User satisfaction with auto mode > 75%
- Network efficiency: bandwidth usage within 10% of ideal

---

#### **Phase 3: Multi-Codec Support** (Week 5-6)
**Goal**: Support modern codecs for bandwidth savings

**Tasks**:
1. Codec capability detection (frontend)
   - Media Capabilities API integration
   - Browser-specific codec detection
   - Hardware acceleration detection

2. Multi-codec transcode profiles (backend)
   - H.265 profiles for Safari/Edge
   - VP9 profiles for Chrome/Firefox
   - Codec selection logic

3. FFmpeg multi-codec support
   - H.265 encoder configuration
   - VP9 encoder configuration
   - Hardware acceleration (NVENC, QSV, VideoToolbox)

4. Codec negotiation
   - Accept header parsing
   - Codec preference ordering
   - Fallback strategy

5. Testing infrastructure
   - Multi-codec test suite
   - Browser compatibility matrix
   - Performance benchmarking

**Deliverables**:
- ✅ H.265 support for Apple devices (40% bandwidth savings)
- ✅ VP9 support for Chrome/Firefox (35% bandwidth savings)
- ✅ Automatic codec selection based on client
- ✅ Fallback to H.264 for compatibility

**Success Metrics**:
- Bandwidth savings: 30-40% for modern browsers
- Encode time increase < 50%
- Zero playback failures from codec issues
- Hardware acceleration usage > 70% where available

---

#### **Phase 4: Legacy System Migration** (Week 7)

**Goal**: Deprecate and remove legacy DASH-based transcoding system

**Background**: The codebase currently contains two parallel transcoding systems:

- **Legacy**: `QualityProfile` in `profiles.go` with 4 basic profiles (360p, 720p, 1080p, 4K) and string-based bitrates
- **New**: `AdaptiveProfile` in `adaptive_profiles.go` with 34 granular profiles and comprehensive metadata

This phase consolidates to a single HLS-based system using the new adaptive profiles.

**Tasks**:

1. **Create backward compatibility layer**
   - Add `GetProfileForLegacyQuality()` function to map old quality strings to new profile IDs
   - Map: 360p → Quality360p800k, 720p → Quality720p2500k, 1080p → Quality1080p5000k, 4k → Quality4K15000k

2. **Migrate transcoding infrastructure**
   - Update `job_executor.go` to use AdaptiveProfile instead of QualityProfile
   - Update `ffmpeg_args_builder.go` to accept integer bitrates instead of string bitrates
   - Update `session_manager.go` to use new profile lookups
   - Update `serve_manifest.go` to use adaptive profile system

3. **Remove legacy code**
   - Delete `profiles.go` entirely
   - Remove `GetQualityProfile()` function
   - Clean up any remaining references to old profile system

4. **Update domain layer**
   - Keep existing quality constants (Quality360p, Quality720p, Quality1080p, Quality4K) for API compatibility
   - Ensure `isValidQuality()` continues to work with 4 standard quality levels
   - Domain layer remains stable, infrastructure layer handles mapping

5. **Testing**
   - Verify all existing transcode jobs work with new profiles
   - Test all streaming strategies (DirectPlay, Remux, RemuxWithAudio, Transcode)
   - Ensure no regressions in video playback
   - Validate session management with new profiles

**Deliverables**:

- ✅ Single unified HLS-based transcoding system
- ✅ All legacy code removed
- ✅ Backward compatibility maintained at API level
- ✅ Zero regressions in existing functionality

**Success Metrics**:

- All transcode jobs complete successfully with new profiles
- No increase in failure rates
- Code complexity reduced (fewer lines of code)
- Clear separation between domain constants and infrastructure profiles

---

#### **Phase 5: Advanced Features** (Week 8-9)
**Goal**: Polish, optimization, and future-proofing

**Tasks**:
1. AV1 codec support (experimental)
   - AV1 encoder integration
   - Chrome 90+ detection
   - Bandwidth savings measurement

2. Analytics dashboard
   - Quality distribution charts
   - Buffer event visualization
   - Recommendation accuracy tracking
   - User override patterns

3. A/B testing framework
   - Test different recommendation algorithms
   - Test different quality profiles
   - Measure impact on user engagement

4. Performance optimizations
   - Reduce recommendation latency
   - Optimize speed test overhead
   - Cache recommendation results

5. Documentation
   - API documentation
   - User guide for quality selection
   - Admin guide for profile tuning
   - Architecture diagrams

**Deliverables**:
- ✅ AV1 support for cutting-edge browsers (50% bandwidth savings)
- ✅ Comprehensive analytics for optimization
- ✅ A/B testing for continuous improvement
- ✅ Complete documentation

**Success Metrics**:
- Recommendation latency < 200ms
- Analytics coverage for 100% of sessions
- Documentation completeness score > 90%
- A/B test framework operational

## Consequences

### Positive Consequences

1. **Better User Experience**
   - Users get optimal quality without manual selection
   - Reduced buffering on poor connections
   - Automatic adaptation to changing conditions
   - Mobile users save data and battery

2. **Bandwidth Efficiency**
   - 30-50% bandwidth savings with modern codecs
   - Reduced server bandwidth costs
   - Less unnecessary transcoding

3. **Future-Proof Architecture**
   - Extensible to new devices (Smart TVs, game consoles)
   - Supports emerging codecs (AV1, future standards)
   - Flexible profile system for new use cases

4. **Data-Driven Optimization**
   - Analytics enable continuous improvement
   - User behavior informs recommendations
   - A/B testing validates changes

5. **Competitive Feature Set**
   - Matches commercial platforms (Netflix, Disney+)
   - Professional adaptive streaming
   - Modern codec support

### Negative Consequences

1. **Increased Complexity**
   - More code to maintain
   - More testing scenarios
   - More configuration options

2. **Backend Processing**
   - Multiple codec transcodes increase storage
   - More CPU usage for encoding
   - Network speed test adds minor load

3. **Client-Side Overhead**
   - Capability detection adds initial delay
   - Network monitoring uses bandwidth
   - More complex player logic

4. **Migration Effort**
   - Existing transcodes may need regeneration
   - Database migrations required
   - User preference migration

### Mitigation Strategies

1. **Complexity Management**
   - Comprehensive test coverage
   - Clear documentation
   - Feature flags for gradual rollout

2. **Resource Optimization**
   - Lazy transcode generation (on-demand)
   - LRU cache eviction for old transcodes
   - Hardware acceleration where available
   - Scheduled cleanup jobs

3. **Performance Optimization**
   - Cache capability detection results
   - Throttle speed tests (max once per minute)
   - Optimize recommendation algorithm
   - CDN for test chunks

4. **User Experience**
   - Progressive enhancement (fallback to current system)
   - Clear UI feedback during transitions
   - User education tooltips
   - Opt-out options for advanced users

## Alternatives Considered

### Alternative 1: Client-Side Transcoding
**Description**: Use WebCodecs API for client-side transcoding

**Pros**:
- No server processing
- Instant quality switching
- No bandwidth for multiple qualities

**Cons**:
- Limited browser support (Chrome 94+)
- High CPU usage on client
- Battery drain on mobile
- Can't leverage server hardware acceleration

**Decision**: Rejected - Too bleeding-edge, poor mobile experience

### Alternative 2: Fixed ABR (Adaptive Bitrate) Ladder
**Description**: Pre-generate all qualities for all videos

**Pros**:
- Simple implementation
- Predictable behavior
- No on-demand transcoding

**Cons**:
- Massive storage requirements (7x per video)
- Wastes resources on rarely-watched content
- Long wait time before playback
- No flexibility for source quality

**Decision**: Rejected - Doesn't scale, wasteful

### Alternative 3: Single Quality with Bitrate Adaptation
**Description**: Use fixed resolution, vary bitrate only

**Pros**:
- Simpler than multiple qualities
- Smooth transitions
- Less storage

**Cons**:
- Doesn't address screen size differences
- Can't optimize for mobile vs desktop
- Misses source quality opportunities

**Decision**: Rejected - Doesn't solve core problems

### Alternative 4: Third-Party Service (Mux, Cloudflare Stream)
**Description**: Offload all transcoding to external service

**Pros**:
- No transcoding infrastructure needed
- Professional quality
- Global CDN included

**Cons**:
- Monthly costs ($5-10 per 1000 minutes)
- Vendor lock-in
- Data privacy concerns
- Not self-hosted

**Decision**: Rejected - Against ViewRA's self-hosted philosophy

## Testing Strategy

### Unit Tests
- Capability detection functions
- Recommendation algorithm logic
- Profile filtering functions
- Network speed calculation

### Integration Tests
- End-to-end recommendation flow
- Quality switching during playback
- Preference persistence
- Analytics collection

### Performance Tests
- Recommendation latency
- Speed test overhead
- Quality switch transition time
- Encoding performance by codec

### Browser Compatibility Tests
- Capability detection across browsers
- Codec support verification
- HLS.js integration
- Mobile browser testing

### User Acceptance Tests
- Quality recommendation accuracy
- Auto mode behavior
- Manual override functionality
- UI clarity and usability

## Monitoring & Metrics

### Key Performance Indicators (KPIs)

1. **Recommendation Accuracy**
   - % of users who accept recommendation
   - % of users who override recommendation
   - Average overrides per session

2. **Quality of Experience (QoE)**
   - Buffer ratio (buffer time / playback time)
   - Stall count per session
   - Quality switches per session
   - Session completion rate

3. **Efficiency**
   - Average bandwidth per quality
   - Bandwidth savings vs fixed H.264
   - Transcode hit rate (cache efficiency)
   - Hardware acceleration usage

4. **Performance**
   - Recommendation latency (p50, p95, p99)
   - Quality switch transition time
   - Speed test overhead
   - Initial playback time

### Monitoring Dashboards

1. **Real-Time Dashboard**
   - Active sessions by quality
   - Current buffer health distribution
   - Network speed distribution
   - Quality switch events (live)

2. **Analytics Dashboard**
   - Quality distribution over time
   - Recommendation accuracy trends
   - Bandwidth savings by codec
   - Device type breakdown

3. **Performance Dashboard**
   - API latency metrics
   - Transcode job queue depth
   - Encoding duration by profile
   - Cache hit rates

## Documentation Requirements

1. **User Documentation**
   - Quality selection guide
   - Understanding recommendations
   - Data saving tips
   - Troubleshooting buffering

2. **Developer Documentation**
   - API reference
   - Capability detection guide
   - Adding new profiles
   - Codec integration guide

3. **Operations Documentation**
   - Profile tuning guide
   - Performance optimization
   - Monitoring & alerting
   - Troubleshooting guide

## References

- [HLS.js Documentation](https://github.com/video-dev/hls.js/)
- [Media Capabilities API](https://developer.mozilla.org/en-US/docs/Web/API/Media_Capabilities_API)
- [FFmpeg Encoding Guide](https://trac.ffmpeg.org/wiki/Encode/H.264)
- [Netflix: Optimizing Streaming QoE](https://netflixtechblog.com/)
- [YouTube: VP9 Video Compression](https://www.youtube.com/intl/en-GB/howyoutubeworks/our-commitments/improving-quality-video/)
- [AV1 Codec Overview](https://aomedia.org/av1/)

## Approval

- **Author**: Claude Code
- **Date**: 2025-11-24
- **Reviewers**: [Pending]
- **Status**: Awaiting approval
