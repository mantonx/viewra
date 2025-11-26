# Project Plan: ADR 0003 Implementation
## Adaptive Video Transcoding System

**Status**: Ready for Execution
**Created**: 2025-11-24
**Total Estimated Time**: 8-9 weeks (280-360 hours)
**Phases**: 5
**Breaking Changes**: Phase 4 removes legacy DASH system (internal only, API compatibility maintained)

---

## Executive Summary

This project plan outlines the complete implementation of ADR 0003, which adds intelligent, adaptive video transcoding to ViewRA. The system will automatically detect client capabilities, recommend optimal video quality, and dynamically adapt during playback based on network conditions.

**Key Objectives**:
- ✅ Implement client capability detection (device, network, screen, battery)
- ✅ Build quality recommendation engine with intelligent algorithm
- ✅ Add granular bitrate-based quality profiles (23 profiles: `240p-400k` through `4k-80000k`)
- ✅ Prioritize direct playback when possible (detect container/audio/video compatibility)
- ✅ Enable efficient container remuxing (MKV → HLS without re-encoding)
- ✅ Enable real-time adaptive quality switching during playback
- ✅ Support modern codecs (H.265, VP9) for 30-40% bandwidth savings (Phase 3)
- ✅ Create analytics pipeline for continuous optimization
- ✅ Maintain backward compatibility with existing playback

**Success Metrics**:
- Recommendation accuracy > 80% (users accept recommendation)
- Buffer rate < 1% of playback time
- Bandwidth savings 30-40% with modern codecs
- Zero regressions in existing playback functionality

---

## Phase 1: Foundation (Week 1-2)
**Duration**: 80-100 hours
**Goal**: Basic capability detection and recommendation system

### Task 1.1: Client Capability Detection Module (16 hours)

**Location**: `web/src/lib/capabilities/`

**Files to Create**:
```
web/src/lib/capabilities/
├── CapabilityDetector.ts         (main detector class)
├── DeviceDetector.ts              (device type detection)
├── NetworkDetector.ts             (network speed & type)
├── MediaCapabilityDetector.ts     (codec support)
├── types.ts                       (TypeScript interfaces)
└── index.ts                       (exports)
```

**Implementation Details**:

**`types.ts`** (2 hours):
```typescript
export interface ClientCapabilities {
  // Network
  networkSpeedMbps: number
  connectionType: 'wifi' | '4g' | '5g' | 'ethernet' | 'slow-2g' | '2g' | '3g' | 'unknown'
  isMetered: boolean
  effectiveType: string // '4g', 'slow-2g', etc.

  // Device
  deviceType: 'mobile' | 'tablet' | 'desktop' | 'tv' | 'unknown'
  screenWidth: number
  screenHeight: number
  pixelRatio: number
  isTouchDevice: boolean

  // Performance
  cpuCores: number
  memoryGB: number
  batteryLevel: number  // 0-1, -1 if not available
  lowPowerMode: boolean
  isCharging: boolean

  // Media Support
  supportedCodecs: string[]
  hardwareAcceleration: boolean
  maxDecodingProfile: string // e.g., "4k-60fps"

  // Browser
  userAgent: string
  browserName: string
  browserVersion: string

  // Timestamp
  detectedAt: Date
}

export interface NetworkMeasurement {
  speedMbps: number
  latencyMs: number
  jitter: number
  timestamp: Date
}
```

**`DeviceDetector.ts`** (3 hours):
```typescript
export class DeviceDetector {
  detectDeviceType(): DeviceType {
    const ua = navigator.userAgent.toLowerCase()
    const width = window.screen.width
    const height = window.screen.height
    const minDimension = Math.min(width, height)

    // TV detection
    if (ua.includes('tv') || ua.includes('smarttv') ||
        ua.includes('googletv') || ua.includes('appletv')) {
      return 'tv'
    }

    // Mobile detection
    if (/android|iphone|ipod|mobile/.test(ua)) {
      return minDimension < 768 ? 'mobile' : 'tablet'
    }

    // iPad detection (tricky since Safari reports as desktop)
    if (/ipad|macintosh/.test(ua) && navigator.maxTouchPoints > 1) {
      return 'tablet'
    }

    // Tablet detection by screen size
    if (minDimension >= 768 && minDimension < 1024 &&
        window.matchMedia('(pointer: coarse)').matches) {
      return 'tablet'
    }

    return 'desktop'
  }

  getScreenInfo() {
    return {
      width: window.screen.width,
      height: window.screen.height,
      pixelRatio: window.devicePixelRatio || 1,
      availWidth: window.screen.availWidth,
      availHeight: window.screen.availHeight,
      orientation: window.screen.orientation?.type || 'unknown'
    }
  }

  getPerformanceInfo() {
    return {
      cpuCores: navigator.hardwareConcurrency || 2,
      memoryGB: (navigator as any).deviceMemory || -1,
      isTouchDevice: 'ontouchstart' in window || navigator.maxTouchPoints > 0
    }
  }

  async getBatteryInfo() {
    if (!('getBattery' in navigator)) {
      return { level: -1, charging: false, chargingTime: -1, dischargingTime: -1 }
    }

    try {
      const battery = await (navigator as any).getBattery()
      return {
        level: battery.level, // 0-1
        charging: battery.charging,
        chargingTime: battery.chargingTime,
        dischargingTime: battery.dischargingTime
      }
    } catch {
      return { level: -1, charging: false, chargingTime: -1, dischargingTime: -1 }
    }
  }

  isLowPowerMode(): boolean {
    // iOS 9+ Low Power Mode detection (indirect)
    // When enabled, requestAnimationFrame throttles to ~30fps
    const battery = (navigator as any).battery
    if (battery?.level < 0.2 && !battery?.charging) {
      return true // Likely in power saving
    }
    return false
  }
}
```

**`NetworkDetector.ts`** (5 hours):
```typescript
export class NetworkDetector {
  private static readonly TEST_CHUNK_SIZE = 500_000 // 500KB
  private measurements: NetworkMeasurement[] = []

  async detectConnectionType(): Promise<ConnectionInfo> {
    const connection = (navigator as any).connection ||
                       (navigator as any).mozConnection ||
                       (navigator as any).webkitConnection

    if (!connection) {
      return {
        type: 'unknown',
        effectiveType: 'unknown',
        downlink: -1,
        rtt: -1,
        saveData: false
      }
    }

    return {
      type: connection.type || 'unknown', // 'wifi', 'cellular', 'ethernet'
      effectiveType: connection.effectiveType || 'unknown', // '4g', '3g', '2g', 'slow-2g'
      downlink: connection.downlink || -1, // Mbps
      rtt: connection.rtt || -1, // ms
      saveData: connection.saveData || false // User has data saver enabled
    }
  }

  async measureNetworkSpeed(): Promise<NetworkMeasurement> {
    const start = performance.now()
    const startTime = Date.now()

    try {
      // Use a test endpoint that returns a known-size chunk
      const response = await fetch('/api/speedtest/chunk', {
        method: 'GET',
        headers: {
          'Cache-Control': 'no-cache',
          'Pragma': 'no-cache'
        }
      })

      if (!response.ok) {
        throw new Error('Speed test failed')
      }

      const blob = await response.blob()
      const duration = (performance.now() - start) / 1000 // seconds
      const speedMbps = (blob.size * 8) / duration / 1_000_000

      // Measure latency from response headers if available
      const serverTiming = response.headers.get('Server-Timing')
      const latencyMs = this.parseLatency(serverTiming) || (performance.now() - start) / 2

      const measurement: NetworkMeasurement = {
        speedMbps: Math.round(speedMbps * 100) / 100,
        latencyMs: Math.round(latencyMs),
        jitter: 0, // Calculate from multiple measurements
        timestamp: new Date(startTime)
      }

      this.measurements.push(measurement)

      // Keep last 10 measurements
      if (this.measurements.length > 10) {
        this.measurements.shift()
      }

      // Calculate jitter if we have multiple measurements
      if (this.measurements.length > 1) {
        measurement.jitter = this.calculateJitter()
      }

      return measurement
    } catch (error) {
      console.error('Network speed measurement failed:', error)
      return {
        speedMbps: -1,
        latencyMs: -1,
        jitter: -1,
        timestamp: new Date()
      }
    }
  }

  private calculateJitter(): number {
    if (this.measurements.length < 2) return 0

    const speeds = this.measurements.map(m => m.speedMbps)
    let jitterSum = 0

    for (let i = 1; i < speeds.length; i++) {
      jitterSum += Math.abs(speeds[i] - speeds[i - 1])
    }

    return jitterSum / (speeds.length - 1)
  }

  getAverageSpeed(): number {
    if (this.measurements.length === 0) return -1

    const sum = this.measurements.reduce((acc, m) => acc + m.speedMbps, 0)
    return sum / this.measurements.length
  }

  private parseLatency(serverTiming: string | null): number | null {
    if (!serverTiming) return null

    // Parse Server-Timing header: "processing;dur=123"
    const match = serverTiming.match(/dur=(\d+\.?\d*)/)
    return match ? parseFloat(match[1]) : null
  }
}
```

**`MediaCapabilityDetector.ts`** (4 hours):
```typescript
export class MediaCapabilityDetector {
  async detectCodecSupport(): Promise<string[]> {
    const codecs = [
      'video/mp4; codecs="avc1.42E01E"',   // H.264 Baseline
      'video/mp4; codecs="avc1.4D401E"',   // H.264 Main
      'video/mp4; codecs="avc1.64001F"',   // H.264 High
      'video/mp4; codecs="hev1.1.6.L93.B0"', // H.265/HEVC
      'video/webm; codecs="vp8"',          // VP8
      'video/webm; codecs="vp9"',          // VP9
      'video/mp4; codecs="av01.0.05M.08"'  // AV1
    ]

    const supported: string[] = []

    for (const codec of codecs) {
      if (this.canPlayCodec(codec)) {
        supported.push(this.extractCodecName(codec))
      }
    }

    return supported
  }

  private canPlayCodec(mimeType: string): boolean {
    const video = document.createElement('video')
    const canPlay = video.canPlayType(mimeType)
    return canPlay === 'probably' || canPlay === 'maybe'
  }

  private extractCodecName(mimeType: string): string {
    // Extract codec name from MIME type
    const match = mimeType.match(/codecs="([^"]+)"/)
    if (!match) return 'unknown'

    const codec = match[1]
    if (codec.startsWith('avc1')) return 'h264'
    if (codec.startsWith('hev1') || codec.startsWith('hvc1')) return 'h265'
    if (codec.startsWith('vp8')) return 'vp8'
    if (codec.startsWith('vp9')) return 'vp9'
    if (codec.startsWith('av01')) return 'av1'
    return 'unknown'
  }

  async detectHardwareAcceleration(): Promise<boolean> {
    // Check if Media Capabilities API is available
    if (!('mediaCapabilities' in navigator)) {
      return false // Assume no HW accel if API not available
    }

    try {
      // Test with a common configuration
      const config = {
        type: 'file' as const,
        video: {
          contentType: 'video/mp4; codecs="avc1.42E01E"',
          width: 1920,
          height: 1080,
          bitrate: 5000000,
          framerate: 30
        }
      }

      const result = await navigator.mediaCapabilities.decodingInfo(config)

      // If smooth and power efficient, likely using HW acceleration
      return result.smooth && result.powerEfficient
    } catch {
      return false
    }
  }

  async detectMaxDecodingProfile(): Promise<string> {
    // Test progressively higher profiles
    const profiles = [
      { width: 1920, height: 1080, fps: 30, name: '1080p-30fps' },
      { width: 1920, height: 1080, fps: 60, name: '1080p-60fps' },
      { width: 3840, height: 2160, fps: 30, name: '4k-30fps' },
      { width: 3840, height: 2160, fps: 60, name: '4k-60fps' }
    ]

    if (!('mediaCapabilities' in navigator)) {
      return '1080p-30fps' // Conservative default
    }

    let maxProfile = '1080p-30fps'

    for (const profile of profiles) {
      try {
        const config = {
          type: 'file' as const,
          video: {
            contentType: 'video/mp4; codecs="avc1.64001F"',
            width: profile.width,
            height: profile.height,
            bitrate: profile.width * profile.height * profile.fps / 10, // Rough estimate
            framerate: profile.fps
          }
        }

        const result = await navigator.mediaCapabilities.decodingInfo(config)

        if (result.smooth) {
          maxProfile = profile.name
        } else {
          break // Stop at first non-smooth profile
        }
      } catch {
        break
      }
    }

    return maxProfile
  }
}
```

**`CapabilityDetector.ts`** (2 hours):
```typescript
import { DeviceDetector } from './DeviceDetector'
import { NetworkDetector } from './NetworkDetector'
import { MediaCapabilityDetector } from './MediaCapabilityDetector'
import type { ClientCapabilities } from './types'

export class CapabilityDetector {
  private deviceDetector: DeviceDetector
  private networkDetector: NetworkDetector
  private mediaDetector: MediaCapabilityDetector
  private cachedCapabilities: ClientCapabilities | null = null
  private cacheExpiry: number = 5 * 60 * 1000 // 5 minutes

  constructor() {
    this.deviceDetector = new DeviceDetector()
    this.networkDetector = new NetworkDetector()
    this.mediaDetector = new MediaCapabilityDetector()
  }

  async detectCapabilities(forceRefresh: boolean = false): Promise<ClientCapabilities> {
    // Return cached if still valid and not forcing refresh
    if (!forceRefresh && this.cachedCapabilities &&
        Date.now() - this.cachedCapabilities.detectedAt.getTime() < this.cacheExpiry) {
      return this.cachedCapabilities
    }

    // Parallel detection for speed
    const [
      deviceType,
      screenInfo,
      performanceInfo,
      batteryInfo,
      connectionInfo,
      networkSpeed,
      supportedCodecs,
      hardwareAccel,
      maxProfile
    ] = await Promise.all([
      Promise.resolve(this.deviceDetector.detectDeviceType()),
      Promise.resolve(this.deviceDetector.getScreenInfo()),
      Promise.resolve(this.deviceDetector.getPerformanceInfo()),
      this.deviceDetector.getBatteryInfo(),
      this.networkDetector.detectConnectionType(),
      this.networkDetector.measureNetworkSpeed(),
      this.mediaDetector.detectCodecSupport(),
      this.mediaDetector.detectHardwareAcceleration(),
      this.mediaDetector.detectMaxDecodingProfile()
    ])

    const capabilities: ClientCapabilities = {
      // Network
      networkSpeedMbps: networkSpeed.speedMbps,
      connectionType: this.mapConnectionType(connectionInfo.type),
      isMetered: connectionInfo.saveData || connectionInfo.type === 'cellular',
      effectiveType: connectionInfo.effectiveType,

      // Device
      deviceType,
      screenWidth: screenInfo.width,
      screenHeight: screenInfo.height,
      pixelRatio: screenInfo.pixelRatio,
      isTouchDevice: performanceInfo.isTouchDevice,

      // Performance
      cpuCores: performanceInfo.cpuCores,
      memoryGB: performanceInfo.memoryGB,
      batteryLevel: batteryInfo.level,
      lowPowerMode: this.deviceDetector.isLowPowerMode(),
      isCharging: batteryInfo.charging,

      // Media
      supportedCodecs,
      hardwareAcceleration: hardwareAccel,
      maxDecodingProfile: maxProfile,

      // Browser
      userAgent: navigator.userAgent,
      browserName: this.detectBrowser(),
      browserVersion: this.detectBrowserVersion(),

      // Metadata
      detectedAt: new Date()
    }

    this.cachedCapabilities = capabilities
    return capabilities
  }

  private mapConnectionType(type: string): ClientCapabilities['connectionType'] {
    const typeMap: Record<string, ClientCapabilities['connectionType']> = {
      'wifi': 'wifi',
      'ethernet': 'ethernet',
      'cellular': '4g',
      '4g': '4g',
      '5g': '5g',
      '3g': '3g',
      '2g': '2g',
      'slow-2g': 'slow-2g'
    }
    return typeMap[type] || 'unknown'
  }

  private detectBrowser(): string {
    const ua = navigator.userAgent.toLowerCase()
    if (ua.includes('firefox')) return 'firefox'
    if (ua.includes('edg/')) return 'edge'
    if (ua.includes('chrome')) return 'chrome'
    if (ua.includes('safari')) return 'safari'
    return 'unknown'
  }

  private detectBrowserVersion(): string {
    const ua = navigator.userAgent
    const match = ua.match(/(firefox|chrome|safari|edg)\/(\d+)/i)
    return match ? match[2] : 'unknown'
  }

  // Convenience method for monitoring network over time
  async startNetworkMonitoring(intervalMs: number = 60000, callback: (speed: number) => void) {
    const monitor = async () => {
      const measurement = await this.networkDetector.measureNetworkSpeed()
      callback(measurement.speedMbps)
    }

    // Initial measurement
    await monitor()

    // Periodic measurements
    return setInterval(monitor, intervalMs)
  }
}

// Singleton export for convenience
export const capabilityDetector = new CapabilityDetector()
```

**Testing** (1 hour):
- Unit tests for each detector
- Integration test for full capability detection
- Browser compatibility tests

---

### Task 1.2: Backend Quality Profiles Extension (12 hours)

**Location**: `internal/infrastructure/transcoding/profiles.go`

**Granular Quality Profile IDs** (1 hour):
```go
// Granular bitrate-based quality profile IDs
// Format: "{resolution}-{bitrate}" (e.g., "1080p-8000k", "4k-40000k")
// This gives users precise control over bandwidth vs quality tradeoff
const (
    // 240p - Ultra Low
    Quality240p400k  = "240p-400k"

    // 360p - Low
    Quality360p800k  = "360p-800k"

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

    // 4K - Ultra HD (5 variants)
    Quality4k16000k = "4k-16000k"
    Quality4k25000k = "4k-25000k"
    Quality4k35000k = "4k-35000k"
    Quality4k50000k = "4k-50000k"
    Quality4k80000k = "4k-80000k"
)

// Total: 23 granular profiles
```

**Enhanced Profile Structure** (3 hours):
```go
// AdaptiveProfile for granular bitrate-based quality profiles
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
    AudioBitrate    int // bits per second
    AudioChannels   int
    AudioSampleRate int

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

**New Profile Definitions** (4 hours):
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

    // 1080p - Full HD (5 variants showing granular control)
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

    // 4K variants (showing even more granular control for premium quality)
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
        PreferredCodec:  "h265", // H.265 recommended for 4K
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

    // NOTE: Total of 23 profiles as defined in ADR 0003
    // Complete profile catalog in ADR document (lines 299-934)
}
```

**Helper Functions** (2 hours):
```go
func GetAdaptiveProfile(quality string) (*AdaptiveProfile, error) {
    profile, exists := adaptiveProfiles[quality]
    if !exists {
        return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
    }
    return profile, nil
}

func GetAllAdaptiveProfiles() []*AdaptiveProfile {
    profiles := make([]*AdaptiveProfile, 0, len(adaptiveProfiles))
    for _, profile := range adaptiveProfiles {
        profiles = append(profiles, profile)
    }
    return profiles
}

func FilterProfilesByScreenSize(profiles []*AdaptiveProfile, width, height int) []*AdaptiveProfile {
    filtered := []*AdaptiveProfile{}
    for _, profile := range profiles {
        if profile.Width <= width && profile.Height <= height {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}

func FilterProfilesByNetworkSpeed(profiles []*AdaptiveProfile, speedMbps float64) []*AdaptiveProfile {
    filtered := []*AdaptiveProfile{}
    for _, profile := range profiles {
        if profile.MinNetworkMbps <= speedMbps {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}
```

**Database Migration** (2 hours):
```sql
-- Migration: 000020_add_adaptive_transcode_fields.up.sql

-- Add codec support to transcode_jobs
ALTER TABLE transcode_jobs
    ADD COLUMN IF NOT EXISTS codec VARCHAR(10) DEFAULT 'h264',
    ADD COLUMN IF NOT EXISTS client_device_type VARCHAR(20),
    ADD COLUMN IF NOT EXISTS client_network_type VARCHAR(20),
    ADD COLUMN IF NOT EXISTS recommended_quality VARCHAR(10);

CREATE INDEX IF NOT EXISTS idx_transcode_jobs_codec
    ON transcode_jobs(codec);
CREATE INDEX IF NOT EXISTS idx_transcode_jobs_device_type
    ON transcode_jobs(client_device_type);

-- Add user preferences table
CREATE TABLE IF NOT EXISTS user_video_preferences (
    user_id BIGINT PRIMARY KEY,
    quality_preference VARCHAR(10),
    prefer_data_saving BOOLEAN DEFAULT false,
    prefer_quality BOOLEAN DEFAULT false,
    allow_cellular_hd BOOLEAN DEFAULT false,
    allow_cellular_4k BOOLEAN DEFAULT false,
    preferred_codec VARCHAR(10),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Down migration
-- 000020_add_adaptive_transcode_fields.down.sql
ALTER TABLE transcode_jobs
    DROP COLUMN IF EXISTS codec,
    DROP COLUMN IF EXISTS client_device_type,
    DROP COLUMN IF EXISTS client_network_type,
    DROP COLUMN IF EXISTS recommended_quality;

DROP INDEX IF EXISTS idx_transcode_jobs_codec;
DROP INDEX IF EXISTS idx_transcode_jobs_device_type;

DROP TABLE IF EXISTS user_video_preferences;
```

---

### Task 1.3: Quality Recommendation Engine (20 hours)

**Location**: `internal/application/transcode/recommend_quality.go`

**Request/Response Types** (2 hours):
```go
package transcode

import (
    "time"
)

type QualityRecommendationRequest struct {
    // Network capabilities
    NetworkSpeed      float64  `json:"network_speed_mbps" binding:"required"`
    ConnectionType    string   `json:"connection_type" binding:"required"`
    IsMetered         bool     `json:"is_metered"`
    EffectiveType     string   `json:"effective_type"`

    // Device capabilities
    DeviceType        string   `json:"device_type" binding:"required"`
    ScreenWidth       int      `json:"screen_width" binding:"required"`
    ScreenHeight      int      `json:"screen_height" binding:"required"`
    PixelRatio        float64  `json:"pixel_ratio"`

    // Performance
    CPUCores          int      `json:"cpu_cores"`
    MemoryGB          float64  `json:"memory_gb"`
    BatteryLevel      float64  `json:"battery_level"`     // -1 if unavailable
    LowPowerMode      bool     `json:"low_power_mode"`
    IsCharging        bool     `json:"is_charging"`

    // Media support
    SupportedCodecs   []string `json:"supported_codecs"`
    HWAcceleration    bool     `json:"hardware_acceleration"`

    // User preferences
    PreferDataSaving  bool     `json:"prefer_data_saving"`
    PreferQuality     bool     `json:"prefer_quality"`
    ManualQuality     string   `json:"manual_quality,omitempty"`

    // Context
    MediaID           int64    `json:"media_id"`
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
    Description         string  `json:"description"`
}

type QualityRecommendationResponse struct {
    RecommendedQuality string          `json:"recommended_quality"`
    AvailableQualities []string        `json:"available_qualities"`
    QualityOptions     []QualityOption `json:"quality_options"`
    Reasoning          string          `json:"reasoning"`
    SourceInfo         *SourceInfo     `json:"source_info,omitempty"`
}

type SourceInfo struct {
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    Codec      string `json:"codec"`
    Bitrate    int    `json:"bitrate"`
    Duration   int    `json:"duration"`
    Compatible bool   `json:"compatible"` // Can direct play
}
```

**Core Recommendation Logic** (12 hours):
```go
// RecommendQuality analyzes request and returns optimal quality recommendation
func (uc *RecommendQualityUseCase) Execute(
    ctx context.Context,
    req QualityRecommendationRequest,
) (*QualityRecommendationResponse, error) {
    // 1. Get media information (resolution, codec, bitrate)
    mediaInfo, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
    if err != nil {
        return nil, fmt.Errorf("failed to get media info: %w", err)
    }

    // 2. Get video technical details
    videoInfo, err := uc.videoInfoService.GetVideoInfo(mediaInfo.FilePath)
    if err != nil {
        // Continue without video info, but log warning
        uc.logger.Warn("failed to get video info", "path", mediaInfo.FilePath, "error", err)
    }

    // 3. Get all available profiles
    allProfiles := transcoding.GetAllAdaptiveProfiles()

    // 4. Apply filters
    profiles := uc.filterProfiles(allProfiles, req, videoInfo)

    // 5. Score each profile
    scored := uc.scoreProfiles(profiles, req, videoInfo)

    // 6. Select best match
    recommended := uc.selectBestProfile(scored)

    // 7. Build response
    return uc.buildResponse(recommended, scored, req, videoInfo)
}

// filterProfiles applies hard filters (screen size, network, source resolution)
func (uc *RecommendQualityUseCase) filterProfiles(
    profiles []*transcoding.AdaptiveProfile,
    req QualityRecommendationRequest,
    videoInfo *transcoding.VideoInfo,
) []*transcoding.AdaptiveProfile {
    filtered := profiles

    // Filter 1: Screen resolution (accounting for pixel ratio)
    effectiveWidth := int(float64(req.ScreenWidth) * req.PixelRatio)
    effectiveHeight := int(float64(req.ScreenHeight) * req.PixelRatio)
    filtered = filterByScreenSize(filtered, effectiveWidth, effectiveHeight)

    // Filter 2: Network speed (with buffer for variance)
    // Add 20% buffer to account for network fluctuations
    minSpeed := req.NetworkSpeed * 0.8
    filtered = filterByNetworkSpeed(filtered, minSpeed)

    // Filter 3: Source resolution (don't upscale)
    if videoInfo != nil && videoInfo.Width > 0 && videoInfo.Height > 0 {
        filtered = filterBySourceResolution(filtered, videoInfo.Width, videoInfo.Height)
    }

    // Filter 4: Low power mode constraints
    if req.LowPowerMode || (req.BatteryLevel > 0 && req.BatteryLevel < 0.2 && !req.IsCharging) {
        filtered = capQuality(filtered, "480p")
    }

    // Filter 5: Metered connection constraints (unless user prefers quality)
    if req.IsMetered && !req.PreferQuality {
        filtered = capQuality(filtered, "720p")
    }

    return filtered
}

// scoreProfiles assigns a score to each profile based on suitability
func (uc *RecommendQualityUseCase) scoreProfiles(
    profiles []*transcoding.AdaptiveProfile,
    req QualityRecommendationRequest,
    videoInfo *transcoding.VideoInfo,
) []ScoredProfile {
    scored := make([]ScoredProfile, 0, len(profiles))

    for _, profile := range profiles {
        score := 0.0

        // Score based on device type match
        for _, recommendedFor := range profile.RecommendedFor {
            if recommendedFor == req.DeviceType {
                score += 30.0
                break
            }
        }

        // Score based on network headroom
        // Prefer profiles that use 60-80% of available bandwidth
        networkUtilization := profile.MinNetworkMbps / req.NetworkSpeed
        if networkUtilization >= 0.6 && networkUtilization <= 0.8 {
            score += 25.0
        } else if networkUtilization < 0.6 {
            score += 15.0 // Some headroom is good
        } else {
            score += 5.0 // Using too much bandwidth
        }

        // Score based on screen size utilization
        // Prefer profiles that use 80-100% of screen resolution
        screenUtilization := float64(profile.Height) / float64(req.ScreenHeight)
        if screenUtilization >= 0.8 && screenUtilization <= 1.0 {
            score += 20.0
        } else if screenUtilization >= 0.6 && screenUtilization < 0.8 {
            score += 10.0
        }

        // Score based on source match
        if videoInfo != nil && videoInfo.Height > 0 {
            if profile.Height == videoInfo.Height {
                score += 15.0 // Perfect match
            } else if profile.Height < videoInfo.Height {
                heightRatio := float64(profile.Height) / float64(videoInfo.Height)
                score += 10.0 * heightRatio // Closer to source is better
            }
        }

        // Adjust for user preferences
        if req.PreferDataSaving {
            // Bias toward lower qualities
            qualityPenalty := float64(profile.Height) / 1000.0
            score -= qualityPenalty * 5
        } else if req.PreferQuality {
            // Bias toward higher qualities
            qualityBonus := float64(profile.Height) / 1000.0
            score += qualityBonus * 5
        }

        // Penalty for poor network on high quality
        if profile.MinNetworkMbps > req.NetworkSpeed * 0.9 {
            score -= 10.0 // Too close to network limit
        }

        scored = append(scored, ScoredProfile{
            Profile: profile,
            Score:   score,
        })
    }

    // Sort by score descending
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    return scored
}

// selectBestProfile chooses the highest-scoring profile
func (uc *RecommendQualityUseCase) selectBestProfile(scored []ScoredProfile) *transcoding.AdaptiveProfile {
    if len(scored) == 0 {
        // Fallback to 360p if no profiles match
        profile, _ := transcoding.GetAdaptiveProfile("360p")
        return profile
    }

    return scored[0].Profile
}

// buildResponse constructs the API response
func (uc *RecommendQualityUseCase) buildResponse(
    recommended *transcoding.AdaptiveProfile,
    scored []ScoredProfile,
    req QualityRecommendationRequest,
    videoInfo *transcoding.VideoInfo,
) *QualityRecommendationResponse {
    options := make([]QualityOption, 0, len(scored))
    qualities := make([]string, 0, len(scored))

    for _, sp := range scored {
        profile := sp.Profile

        // Check if can direct play
        canDirectPlay := false
        needsTranscode := true
        if videoInfo != nil {
            strategy, _ := transcoding.DetermineStreamStrategy(videoInfo)
            canDirectPlay = strategy == transcoding.DirectPlay
            needsTranscode = strategy == transcoding.Transcode
        }

        option := QualityOption{
            Quality:             profile.Quality,
            Resolution:          fmt.Sprintf("%dx%d", profile.Width, profile.Height),
            EstimatedBitrate:    profile.TargetBitrate,
            RequiredNetworkMbps: profile.MinNetworkMbps,
            DataUsagePerHour:    fmt.Sprintf("%d MB", profile.DataUsageMBPerHour),
            IsRecommended:       profile.Quality == recommended.Quality,
            CanDirectPlay:       canDirectPlay && profile.Quality == videoInfo.Quality,
            NeedsTranscode:      needsTranscode,
            PreferredCodec:      profile.PreferredCodec,
            Description:         profile.Description,
        }

        options = append(options, option)
        qualities = append(qualities, profile.Quality)
    }

    // Generate reasoning
    reasoning := uc.generateReasoning(recommended, req, videoInfo)

    response := &QualityRecommendationResponse{
        RecommendedQuality: recommended.Quality,
        AvailableQualities: qualities,
        QualityOptions:     options,
        Reasoning:          reasoning,
    }

    // Add source info if available
    if videoInfo != nil {
        response.SourceInfo = &SourceInfo{
            Width:      videoInfo.Width,
            Height:     videoInfo.Height,
            Codec:      videoInfo.Codec,
            Bitrate:    videoInfo.Bitrate,
            Duration:   int(videoInfo.Duration),
            Compatible: videoInfo.Codec == "h264" && videoInfo.AudioChannels <= 2,
        }
    }

    return response
}

// generateReasoning creates human-readable explanation
func (uc *RecommendQualityUseCase) generateReasoning(
    profile *transcoding.AdaptiveProfile,
    req QualityRecommendationRequest,
    videoInfo *transcoding.VideoInfo,
) string {
    reasons := []string{}

    // Network reason
    if req.NetworkSpeed >= profile.MinNetworkMbps * 1.5 {
        reasons = append(reasons, fmt.Sprintf(
            "Your %.1f Mbps %s connection easily supports %s",
            req.NetworkSpeed, req.ConnectionType, profile.Quality,
        ))
    } else if req.NetworkSpeed >= profile.MinNetworkMbps {
        reasons = append(reasons, fmt.Sprintf(
            "Your %.1f Mbps %s connection is suitable for %s",
            req.NetworkSpeed, req.ConnectionType, profile.Quality,
        ))
    }

    // Screen reason
    if req.ScreenHeight >= profile.Height {
        reasons = append(reasons, fmt.Sprintf(
            "Your %dx%d screen displays %s clearly",
            req.ScreenWidth, req.ScreenHeight, profile.Quality,
        ))
    }

    // Device reason
    for _, deviceType := range profile.RecommendedFor {
        if deviceType == req.DeviceType {
            reasons = append(reasons, fmt.Sprintf(
                "%s quality is optimized for %s devices",
                profile.Quality, req.DeviceType,
            ))
            break
        }
    }

    // Source reason
    if videoInfo != nil {
        if videoInfo.Height == profile.Height {
            reasons = append(reasons, "Matches source resolution perfectly")
        } else if videoInfo.Height < profile.Height {
            reasons = append(reasons, fmt.Sprintf(
                "Source is %dp, limited to native quality",
                videoInfo.Height,
            ))
        }
    }

    // Power saving reason
    if req.LowPowerMode {
        reasons = append(reasons, "Limited due to low power mode")
    } else if req.BatteryLevel > 0 && req.BatteryLevel < 0.3 && !req.IsCharging {
        reasons = append(reasons, fmt.Sprintf(
            "Limited to preserve battery (%.0f%% remaining)",
            req.BatteryLevel * 100,
        ))
    }

    // Metered connection reason
    if req.IsMetered && !req.PreferQuality {
        reasons = append(reasons, fmt.Sprintf(
            "Capped to %s to save data on metered connection",
            profile.Quality,
        ))
    }

    if len(reasons) == 0 {
        return fmt.Sprintf("%s recommended based on your device capabilities", profile.Quality)
    }

    return strings.Join(reasons, ". ") + "."
}

type ScoredProfile struct {
    Profile *transcoding.AdaptiveProfile
    Score   float64
}
```

**Helper Functions** (4 hours):
```go
func filterByScreenSize(profiles []*transcoding.AdaptiveProfile, width, height int) []*transcoding.AdaptiveProfile {
    filtered := []*transcoding.AdaptiveProfile{}
    for _, profile := range profiles {
        // Don't offer qualities higher than screen resolution
        if profile.Width <= width && profile.Height <= height {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}

func filterByNetworkSpeed(profiles []*transcoding.AdaptiveProfile, speedMbps float64) []*transcoding.AdaptiveProfile {
    filtered := []*transcoding.AdaptiveProfile{}
    for _, profile := range profiles {
        if profile.MinNetworkMbps <= speedMbps {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}

func filterBySourceResolution(profiles []*transcoding.AdaptiveProfile, sourceWidth, sourceHeight int) []*transcoding.AdaptiveProfile {
    filtered := []*transcoding.AdaptiveProfile{}
    for _, profile := range profiles {
        // Don't upscale - only offer qualities up to source resolution
        if profile.Height <= sourceHeight {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}

func capQuality(profiles []*transcoding.AdaptiveProfile, maxQuality string) []*transcoding.AdaptiveProfile {
    maxProfile, err := transcoding.GetAdaptiveProfile(maxQuality)
    if err != nil {
        return profiles // Return unchanged if max quality invalid
    }

    filtered := []*transcoding.AdaptiveProfile{}
    for _, profile := range profiles {
        if profile.Height <= maxProfile.Height {
            filtered = append(filtered, profile)
        }
    }
    return filtered
}
```

**Testing** (2 hours):
- Unit tests for filtering functions
- Unit tests for scoring logic
- Integration tests for full recommendation flow
- Test cases for edge cases (poor network, low battery, etc.)

---

### Task 1.4: API Endpoint Implementation (16 hours)

**Handler** (6 hours):
```go
// Location: internal/api/handlers/transcode.go

// RecommendQuality returns quality recommendation for given client capabilities
//
// @Summary Recommend video quality
// @Description Analyzes client capabilities and returns optimal quality recommendation
// @Tags transcode
// @Accept json
// @Produce json
// @Param media_id path int true "Media ID"
// @Param request body transcode.QualityRecommendationRequest true "Client capabilities"
// @Success 200 {object} transcode.QualityRecommendationResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/recommend-quality [post]
func (h *TranscodeHandler) RecommendQuality(c *gin.Context) {
    mediaIDStr := c.Param("id")
    mediaID, err := parseID(mediaIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
        return
    }

    var req transcode.QualityRecommendationRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error:   "Invalid request",
            Message: err.Error(),
        })
        return
    }

    // Set media ID from path
    req.MediaID = mediaID

    // Validate request
    if err := validateRecommendationRequest(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error:   "Invalid request parameters",
            Message: err.Error(),
        })
        return
    }

    // Execute use case
    response, err := h.recommendQualityUseCase.Execute(c.Request.Context(), req)
    if err != nil {
        h.logger.Error("failed to recommend quality",
            "media_id", mediaID,
            "error", err)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to generate quality recommendation",
        })
        return
    }

    c.JSON(http.StatusOK, response)
}

func validateRecommendationRequest(req *transcode.QualityRecommendationRequest) error {
    if req.NetworkSpeed < 0 {
        return errors.New("network_speed_mbps must be non-negative")
    }
    if req.ScreenWidth <= 0 || req.ScreenHeight <= 0 {
        return errors.New("screen dimensions must be positive")
    }
    if req.PixelRatio <= 0 {
        req.PixelRatio = 1.0 // Default
    }
    if req.DeviceType == "" {
        req.DeviceType = "desktop" // Default
    }
    if req.ConnectionType == "" {
        req.ConnectionType = "unknown" // Default
    }
    return nil
}
```

**Speed Test Endpoint** (4 hours):
```go
// ServeSpeedTestChunk serves a test chunk for network speed measurement
//
// @Summary Speed test chunk
// @Description Returns a 500KB chunk for client-side speed measurement
// @Tags transcode
// @Produce application/octet-stream
// @Success 200 {file} binary "Test chunk"
// @Router /api/speedtest/chunk [get]
func (h *TranscodeHandler) ServeSpeedTestChunk(c *gin.Context) {
    const chunkSize = 500 * 1024 // 500KB

    // Generate random data (fast, no need to read from disk)
    // In production, consider using a static file to save CPU
    chunk := make([]byte, chunkSize)
    rand.Read(chunk)

    // Set headers
    c.Header("Content-Type", "application/octet-stream")
    c.Header("Content-Length", fmt.Sprintf("%d", chunkSize))
    c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
    c.Header("Pragma", "no-cache")
    c.Header("Expires", "0")

    // Optional: Add server timing for latency measurement
    c.Header("Server-Timing", "processing;dur=0")

    c.Data(http.StatusOK, "application/octet-stream", chunk)
}
```

**Routes** (2 hours):
```go
// Location: internal/api/routes/transcode.go

func RegisterTranscodeRoutes(r *gin.RouterGroup, handler *handlers.TranscodeHandler) {
    // Existing routes...

    // New routes for adaptive streaming
    r.POST("/media/:id/recommend-quality", handler.RecommendQuality)
    r.GET("/speedtest/chunk", handler.ServeSpeedTestChunk)
}
```

**Rate Limiting** (2 hours):
```go
// Add rate limiting for speed test endpoint to prevent abuse
// Location: internal/api/middleware/rate_limit.go

func SpeedTestRateLimiter() gin.HandlerFunc {
    // Allow max 1 request per minute per IP
    limiter := rate.NewLimiter(rate.Every(60*time.Second), 1)

    return func(c *gin.Context) {
        clientIP := c.ClientIP()

        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, handlers.ErrorResponse{
                Error: "Rate limit exceeded for speed test",
                Message: "Please wait before testing again",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

**Testing** (2 hours):
- API integration tests
- Test recommendation endpoint with various inputs
- Test speed test endpoint
- Test rate limiting

---

### Task 1.5: Frontend Integration (20 hours)

**CRITICAL PREREQUISITE: Master Playlist Implementation** (4 hours)

Before frontend integration can be fully functional, the backend must generate HLS master playlists that list all available quality levels.

**Why Required**:

- Current system serves single-quality playlists (`/api/media/:id/hls/720p/playlist.m3u8`)
- HLS.js cannot access multiple quality levels from a single-quality playlist
- Quality recommendations cannot be applied without a multi-quality manifest
- Manual quality switching is impossible without all levels being exposed to the player

**Backend Implementation** (4 hours):

**Location**: `internal/api/handlers/transcode.go` + `internal/api/routes/transcode.go`

```go
// Handler: ServeAdaptiveMasterPlaylist
// Route: GET /api/media/:id/hls/master.m3u8?start=X
func (h *TranscodeHandler) ServeAdaptiveMasterPlaylist(c *gin.Context) {
    mediaID := c.Param("id")
    startTime := c.Query("start") // Resume position in seconds

    // 1. Get all available quality profiles
    profiles := getAllAvailableProfiles() // Returns [360p, 720p, 1080p, 4k, etc.]

    // 2. Generate HLS master playlist
    playlist := "#EXTM3U\n"
    playlist += "#EXT-X-VERSION:3\n\n"

    for _, profile := range profiles {
        // EXT-X-STREAM-INF with bandwidth and resolution
        playlist += fmt.Sprintf(
            "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\"\n",
            profile.VideoBitrate + profile.AudioBitrate,
            profile.Width,
            profile.Height,
            "avc1.64001f,mp4a.40.2", // H.264 High + AAC
        )

        // Variant stream URL (with resume parameter if present)
        variantURL := fmt.Sprintf("%s/playlist.m3u8", profile.Quality)
        if startTime != "" {
            variantURL += "?start=" + startTime
        }
        playlist += variantURL + "\n\n"
    }

    c.Header("Content-Type", "application/vnd.apple.mpegurl")
    c.Header("Cache-Control", "no-cache")
    c.String(http.StatusOK, playlist)
}
```

**Route Registration**:
```go
// Add to internal/api/routes/transcode.go
router.GET("/media/:id/hls/master.m3u8", handler.ServeAdaptiveMasterPlaylist)
```

**Example Output**:

```hls
#EXTM3U
#EXT-X-VERSION:3

#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS="avc1.64001f,mp4a.40.2"
360p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,CODECS="avc1.64001f,mp4a.40.2"
1080p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=15000000,RESOLUTION=3840x2160,CODECS="avc1.64001f,mp4a.40.2"
4k/playlist.m3u8
```

**Frontend Changes**:
```typescript
// Update useMediaPlayback.ts line 70:
// OLD: const manifestUrl = `/api/media/${id}/hls/${quality}/playlist.m3u8`
// NEW: const manifestUrl = `/api/media/${id}/hls/master.m3u8`
```

**Testing**:

- [ ] Master playlist generation with all quality levels
- [ ] Resume parameter propagates to all variant streams
- [ ] HLS.js can parse and switch between quality levels
- [ ] Quality recommendation applies after manifest loads

---

### Task 1.5 (Continued): React Hooks and UI (16 hours)

**React Hook for Recommendations** (4 hours):
```typescript
// Location: web/src/lib/hooks/useQualityRecommendation.ts

import { useEffect, useState } from 'react'
import { capabilityDetector } from '@/lib/capabilities'
import type { ClientCapabilities } from '@/lib/capabilities/types'

interface QualityRecommendation {
  recommendedQuality: string
  availableQualities: string[]
  qualityOptions: QualityOption[]
  reasoning: string
  sourceInfo?: SourceInfo
}

interface QualityOption {
  quality: string
  resolution: string
  estimatedBitrate: string
  requiredNetworkMbps: number
  dataUsagePerHour: string
  isRecommended: boolean
  canDirectPlay: boolean
  needsTranscode: boolean
  preferredCodec: string
  description: string
}

interface SourceInfo {
  width: number
  height: number
  codec: string
  bitrate: number
  duration: number
  compatible: boolean
}

interface UserPreferences {
  preferDataSaving: boolean
  preferQuality: boolean
  manualQuality?: string
}

export function useQualityRecommendation(mediaId: number) {
  const [recommendation, setRecommendation] = useState<QualityRecommendation | null>(null)
  const [capabilities, setCapabilities] = useState<ClientCapabilities | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function fetchRecommendation() {
      try {
        setLoading(true)
        setError(null)

        // 1. Detect client capabilities
        const caps = await capabilityDetector.detectCapabilities()
        setCapabilities(caps)

        // 2. Load user preferences
        const prefs = loadUserPreferences()

        // 3. Request recommendation from server
        const response = await fetch(`/api/media/${mediaId}/recommend-quality`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            // Network
            network_speed_mbps: caps.networkSpeedMbps,
            connection_type: caps.connectionType,
            is_metered: caps.isMetered,
            effective_type: caps.effectiveType,

            // Device
            device_type: caps.deviceType,
            screen_width: caps.screenWidth,
            screen_height: caps.screenHeight,
            pixel_ratio: caps.pixelRatio,

            // Performance
            cpu_cores: caps.cpuCores,
            memory_gb: caps.memoryGB,
            battery_level: caps.batteryLevel,
            low_power_mode: caps.lowPowerMode,
            is_charging: caps.isCharging,

            // Media support
            supported_codecs: caps.supportedCodecs,
            hardware_acceleration: caps.hardwareAcceleration,

            // User preferences
            prefer_data_saving: prefs.preferDataSaving,
            prefer_quality: prefs.preferQuality,
            manual_quality: prefs.manualQuality,
          }),
        })

        if (!response.ok) {
          throw new Error(`Failed to get recommendation: ${response.statusText}`)
        }

        const data = await response.json()
        setRecommendation(data)
      } catch (err) {
        console.error('Failed to get quality recommendation:', err)
        setError(err instanceof Error ? err.message : 'Unknown error')

        // Fallback to default recommendation
        setRecommendation({
          recommendedQuality: '720p',
          availableQualities: ['360p', '720p', '1080p'],
          qualityOptions: [],
          reasoning: 'Using default quality due to detection failure',
        })
      } finally {
        setLoading(false)
      }
    }

    fetchRecommendation()
  }, [mediaId])

  return { recommendation, capabilities, loading, error }
}

function loadUserPreferences(): UserPreferences {
  const stored = localStorage.getItem('video_quality_preferences')
  if (stored) {
    try {
      return JSON.parse(stored)
    } catch {
      return { preferDataSaving: false, preferQuality: false }
    }
  }
  return { preferDataSaving: false, preferQuality: false }
}
```

**Enhanced Quality Selector UI** (6 hours):
```tsx
// Location: web/src/components/media/VideoPlayer/QualitySelector.tsx

import { useState } from 'react'
import { Check, Zap, Wifi, WifiOff, Info } from 'lucide-react'

interface QualitySelectorProps {
  currentQuality: string | null
  recommendedQuality: string
  qualityOptions: QualityOption[]
  networkSpeed: number
  autoMode: boolean
  onQualityChange: (quality: string) => void
  onAutoToggle: () => void
}

export function QualitySelector({
  currentQuality,
  recommendedQuality,
  qualityOptions,
  networkSpeed,
  autoMode,
  onQualityChange,
  onAutoToggle,
}: QualitySelectorProps) {
  const [showTooltip, setShowTooltip] = useState<string | null>(null)

  return (
    <div className="relative">
      <select
        value={autoMode ? 'auto' : (currentQuality || 'auto')}
        onChange={(e) => {
          if (e.target.value === 'auto') {
            onAutoToggle()
          } else {
            onQualityChange(e.target.value)
          }
        }}
        className="bg-white/10 backdrop-blur-sm text-white text-xs sm:text-sm rounded-md px-2 sm:px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-primary-500/50"
        style={{ minWidth: '100px' }}
        aria-label="Video quality"
      >
        {/* Auto option */}
        <option value="auto" className="bg-gray-800">
          Auto {autoMode && currentQuality && `(${currentQuality})`}
        </option>

        {/* Quality options */}
        {qualityOptions.map((option) => (
          <option
            key={option.quality}
            value={option.quality}
            className="bg-gray-800 flex items-center"
          >
            {option.quality} - {option.resolution}
            {option.isRecommended && ' ✓ Recommended'}
            {option.canDirectPlay && ' ⚡ Direct'}
          </option>
        ))}
      </select>

      {/* Expanded quality info panel */}
      {showTooltip && (
        <div className="absolute bottom-full right-0 mb-2 w-72 bg-gray-900/95 backdrop-blur-sm rounded-lg shadow-xl border border-white/10 p-4">
          {qualityOptions
            .filter((opt) => opt.quality === showTooltip)
            .map((option) => (
              <div key={option.quality} className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-white font-semibold">
                    {option.quality}
                  </span>
                  {option.isRecommended && (
                    <span className="flex items-center gap-1 text-green-400 text-xs">
                      <Check className="w-3 h-3" />
                      Recommended
                    </span>
                  )}
                </div>

                <div className="text-sm text-white/70 space-y-1">
                  <div>Resolution: {option.resolution}</div>
                  <div>Bitrate: {option.estimatedBitrate}</div>
                  <div>Data usage: {option.dataUsagePerHour}</div>
                  <div>Required speed: {option.requiredNetworkMbps} Mbps</div>
                </div>

                <div className="flex items-center gap-2 text-xs">
                  {option.canDirectPlay && (
                    <span className="flex items-center gap-1 text-yellow-400">
                      <Zap className="w-3 h-3" />
                      Instant (no transcoding)
                    </span>
                  )}
                  {option.needsTranscode && (
                    <span className="text-white/50">Transcoding required</span>
                  )}
                </div>

                {option.description && (
                  <div className="text-xs text-white/60 pt-2 border-t border-white/10">
                    {option.description}
                  </div>
                )}
              </div>
            ))}
        </div>
      )}
    </div>
  )
}
```

**Update VideoPlayer Integration** (4 hours):
```tsx
// Location: web/src/components/media/VideoPlayer/VideoPlayer.tsx
// Add to imports:
import { useQualityRecommendation } from '@/lib/hooks/useQualityRecommendation'

// Add to component:
export const VideoPlayer = ({ mediaId, ... }: VideoPlayerProps) => {
  // ... existing state ...

  const [autoMode, setAutoMode] = useState(true)
  const { recommendation, loading: recommendationLoading } = useQualityRecommendation(mediaId)

  // Initialize with recommended quality
  useEffect(() => {
    if (recommendation && autoMode && !currentQuality) {
      // Set initial quality to recommended
      setCurrentQuality(recommendation.recommendedQuality)

      // Build stream URL with recommended quality
      const newStreamUrl = `/api/media/${mediaId}/hls/${recommendation.recommendedQuality}/playlist.m3u8`
      // ... set stream URL ...
    }
  }, [recommendation, autoMode, mediaId])

  const handleAutoToggle = () => {
    setAutoMode((prev) => !prev)
    if (!autoMode && recommendation) {
      // Switching to auto - use recommended quality
      handleQualityChange(recommendation.recommendedQuality)
    }
  }

  // ... rest of component ...

  return (
    <div className="fixed inset-0 z-50 bg-black flex flex-col">
      {/* Show loading state while detecting capabilities */}
      {recommendationLoading && (
        <div className="absolute top-20 left-1/2 transform -translate-x-1/2 z-20">
          <div className="bg-primary-500/90 text-white px-4 py-2 rounded-lg">
            Detecting optimal quality...
          </div>
        </div>
      )}

      {/* Show recommendation reasoning */}
      {recommendation && !recommendationLoading && (
        <div className="absolute top-20 left-1/2 transform -translate-x-1/2 z-20 max-w-md">
          <div className="bg-blue-600/90 text-white px-4 py-2 rounded-lg text-sm">
            <Info className="inline w-4 h-4 mr-1" />
            {recommendation.reasoning}
          </div>
        </div>
      )}

      {/* ... rest of player UI ... */}
    </div>
  )
}
```

**User Preferences UI** (2 hours):
```tsx
// Location: web/src/components/media/VideoPlayer/QualityPreferences.tsx

import { useState } from 'react'
import { Save, Database, Award } from 'lucide-react'

export function QualityPreferences() {
  const [preferences, setPreferences] = useState(() => {
    const stored = localStorage.getItem('video_quality_preferences')
    return stored ? JSON.parse(stored) : {
      preferDataSaving: false,
      preferQuality: false,
      allowCellularHD: false,
    }
  })

  const updatePreference = (key: string, value: boolean) => {
    const updated = { ...preferences, [key]: value }
    setPreferences(updated)
    localStorage.setItem('video_quality_preferences', JSON.stringify(updated))
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4 space-y-4">
      <h3 className="text-white font-semibold">Video Quality Preferences</h3>

      <label className="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={preferences.preferDataSaving}
          onChange={(e) => updatePreference('preferDataSaving', e.target.checked)}
          className="w-4 h-4"
        />
        <Database className="w-5 h-5 text-blue-400" />
        <span className="text-white">Prefer data saving</span>
      </label>

      <label className="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={preferences.preferQuality}
          onChange={(e) => updatePreference('preferQuality', e.target.checked)}
          className="w-4 h-4"
        />
        <Award className="w-5 h-5 text-yellow-400" />
        <span className="text-white">Prefer quality</span>
      </label>

      <label className="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={preferences.allowCellularHD}
          onChange={(e) => updatePreference('allowCellularHD', e.target.checked)}
          className="w-4 h-4"
        />
        <span className="text-white">Allow HD on cellular</span>
      </label>
    </div>
  )
}
```

---

### Phase 1 Completion Checklist

- [x] Client capability detection implemented and tested
- [x] 240p, 480p, 1440p profiles added to backend
- [x] Adaptive profile structure created
- [x] Recommendation algorithm implemented
- [x] Recommendation API endpoint created and tested
- [x] Speed test endpoint created with rate limiting
- [ ] Database migrations applied
- [x] Frontend API client for recommendations implemented
- [ ] Frontend hook for recommendations working
- [ ] Quality selector UI enhanced
- [ ] User preferences UI created
- [ ] Integration testing complete
- [ ] Documentation updated
- [ ] Performance metrics baseline established

**Phase 1 Deliverables**:
- ✅ Client can detect device capabilities
- ✅ Server recommends optimal quality with reasoning
- ✅ UI shows recommendation with detailed options
- ✅ User can override recommendation
- ✅ Preferences persist across sessions
- ✅ Database tracks all recommendation data

---

## Phase 2: Adaptive Streaming (Week 3-4)
**Duration**: 80-100 hours
**Goal**: Real-time quality adaptation during playback

### Implementation Note

**Pattern**: All Phase 2 frontend code uses **functional patterns** instead of classes to align with React conventions:

- Pure functions for business logic (e.g., `evaluateQuality`, `calculateStats`)
- Immutable state objects (e.g., `NetworkMonitorState`, `PlaybackAnalyticsState`)
- React hooks for stateful integration (e.g., `useNetworkMonitor`, `useAutoQuality`)

This approach improves testability, aligns with React's functional paradigm, and avoids class-related complexity.

### Task 2.1: Network Monitoring Service (20 hours)

**Location**: `web/src/lib/network/NetworkMonitor.ts`

**Purpose**: Continuously monitor network conditions during playback to enable intelligent quality switching. This service observes actual download performance rather than relying solely on initial capability detection.

**Implementation**:

```typescript
// web/src/lib/network/NetworkMonitor.ts

export interface NetworkSample {
  timestamp: number
  bytesDownloaded: number
  durationMs: number
  throughputMbps: number
  latencyMs: number
  wasStall: boolean
}

export interface NetworkStats {
  currentThroughputMbps: number      // Most recent measurement
  averageThroughputMbps: number      // Rolling average (last 30s)
  minThroughputMbps: number          // Minimum in window
  maxThroughputMbps: number          // Maximum in window
  stability: number                   // 0-1 (1 = very stable)
  stallCount: number                  // Number of buffer underruns
  lastStallTime: number | null       // Timestamp of last stall
  trend: 'improving' | 'stable' | 'degrading'
  connectionType: string              // wifi, cellular, ethernet, etc.
  isMetered: boolean
}

export interface NetworkMonitorConfig {
  sampleWindowMs: number              // How long to keep samples (default: 30000)
  minSamplesForStats: number          // Min samples before reporting (default: 3)
  stabilityThreshold: number          // Variance threshold for stability (default: 0.3)
  onStatsUpdate?: (stats: NetworkStats) => void
  onQualityRecommendation?: (quality: string, reason: string) => void
}

export class NetworkMonitor {
  private samples: NetworkSample[] = []
  private config: NetworkMonitorConfig
  private isMonitoring: boolean = false
  private updateInterval: ReturnType<typeof setInterval> | null = null

  constructor(config: Partial<NetworkMonitorConfig> = {}) {
    this.config = {
      sampleWindowMs: 30000,
      minSamplesForStats: 3,
      stabilityThreshold: 0.3,
      ...config
    }
  }

  // Start monitoring - call when playback begins
  start(): void {
    if (this.isMonitoring) return
    this.isMonitoring = true
    this.samples = []

    // Periodic stats calculation
    this.updateInterval = setInterval(() => {
      this.pruneOldSamples()
      if (this.samples.length >= this.config.minSamplesForStats) {
        const stats = this.calculateStats()
        this.config.onStatsUpdate?.(stats)
        this.checkQualityRecommendation(stats)
      }
    }, 2000)
  }

  // Stop monitoring - call when playback ends
  stop(): void {
    this.isMonitoring = false
    if (this.updateInterval) {
      clearInterval(this.updateInterval)
      this.updateInterval = null
    }
  }

  // Record a network sample - call from HLS.js fragment load events
  recordSample(bytesDownloaded: number, durationMs: number, wasStall: boolean = false): void {
    if (!this.isMonitoring) return

    const throughputMbps = (bytesDownloaded * 8) / (durationMs * 1000) // bits to Mbps

    this.samples.push({
      timestamp: Date.now(),
      bytesDownloaded,
      durationMs,
      throughputMbps,
      latencyMs: 0, // Can be populated from fetch timing
      wasStall
    })
  }

  // Record a buffer stall event
  recordStall(): void {
    if (this.samples.length > 0) {
      this.samples[this.samples.length - 1].wasStall = true
    }
  }

  // Get current network statistics
  getStats(): NetworkStats | null {
    if (this.samples.length < this.config.minSamplesForStats) {
      return null
    }
    return this.calculateStats()
  }

  private pruneOldSamples(): void {
    const cutoff = Date.now() - this.config.sampleWindowMs
    this.samples = this.samples.filter(s => s.timestamp > cutoff)
  }

  private calculateStats(): NetworkStats {
    const throughputs = this.samples.map(s => s.throughputMbps)
    const sum = throughputs.reduce((a, b) => a + b, 0)
    const avg = sum / throughputs.length
    const min = Math.min(...throughputs)
    const max = Math.max(...throughputs)

    // Calculate stability (inverse of coefficient of variation)
    const variance = throughputs.reduce((sum, t) => sum + Math.pow(t - avg, 2), 0) / throughputs.length
    const stdDev = Math.sqrt(variance)
    const cv = avg > 0 ? stdDev / avg : 1
    const stability = Math.max(0, 1 - cv)

    // Count stalls
    const stallCount = this.samples.filter(s => s.wasStall).length
    const lastStall = this.samples.filter(s => s.wasStall).pop()

    // Calculate trend (compare first half to second half)
    const midpoint = Math.floor(this.samples.length / 2)
    const firstHalfAvg = throughputs.slice(0, midpoint).reduce((a, b) => a + b, 0) / midpoint
    const secondHalfAvg = throughputs.slice(midpoint).reduce((a, b) => a + b, 0) / (throughputs.length - midpoint)

    let trend: 'improving' | 'stable' | 'degrading' = 'stable'
    if (secondHalfAvg > firstHalfAvg * 1.15) trend = 'improving'
    else if (secondHalfAvg < firstHalfAvg * 0.85) trend = 'degrading'

    // Get connection info from Navigator API
    const connection = (navigator as any).connection

    return {
      currentThroughputMbps: throughputs[throughputs.length - 1],
      averageThroughputMbps: avg,
      minThroughputMbps: min,
      maxThroughputMbps: max,
      stability,
      stallCount,
      lastStallTime: lastStall?.timestamp ?? null,
      trend,
      connectionType: connection?.effectiveType ?? 'unknown',
      isMetered: connection?.saveData ?? false
    }
  }

  private checkQualityRecommendation(stats: NetworkStats): void {
    // Quality recommendations based on network stats
    // Using conservative bitrate targets (70% of throughput)
    const safeBitrateMbps = stats.averageThroughputMbps * 0.7

    let recommendedQuality: string
    let reason: string

    if (stats.stallCount > 2 || stats.trend === 'degrading') {
      // Network issues - recommend lower quality
      recommendedQuality = safeBitrateMbps < 2 ? '360p' : safeBitrateMbps < 4 ? '480p' : '720p'
      reason = stats.stallCount > 2
        ? `Buffering detected (${stats.stallCount} stalls)`
        : 'Network speed decreasing'
    } else if (stats.stability < 0.5) {
      // Unstable network - be conservative
      recommendedQuality = safeBitrateMbps < 3 ? '480p' : '720p'
      reason = 'Unstable network - using conservative quality'
    } else {
      // Stable network - use normal recommendation
      if (safeBitrateMbps >= 20) recommendedQuality = '4k'
      else if (safeBitrateMbps >= 8) recommendedQuality = '1080p'
      else if (safeBitrateMbps >= 4) recommendedQuality = '720p'
      else if (safeBitrateMbps >= 1.5) recommendedQuality = '480p'
      else recommendedQuality = '360p'
      reason = `Network supports ${stats.averageThroughputMbps.toFixed(1)} Mbps`
    }

    this.config.onQualityRecommendation?.(recommendedQuality, reason)
  }
}

export default NetworkMonitor
```

**React Hook**:

```typescript
// web/src/lib/hooks/useNetworkMonitor.ts

import { useState, useEffect, useRef, useCallback } from 'react'
import { NetworkMonitor, NetworkStats, NetworkMonitorConfig } from '../network/NetworkMonitor'

export interface UseNetworkMonitorOptions {
  enabled?: boolean
  onQualityRecommendation?: (quality: string, reason: string) => void
}

export const useNetworkMonitor = (options: UseNetworkMonitorOptions = {}) => {
  const { enabled = true, onQualityRecommendation } = options
  const [stats, setStats] = useState<NetworkStats | null>(null)
  const [recommendedQuality, setRecommendedQuality] = useState<string | null>(null)
  const monitorRef = useRef<NetworkMonitor | null>(null)

  useEffect(() => {
    if (!enabled) return

    const config: Partial<NetworkMonitorConfig> = {
      onStatsUpdate: setStats,
      onQualityRecommendation: (quality, reason) => {
        setRecommendedQuality(quality)
        onQualityRecommendation?.(quality, reason)
      }
    }

    monitorRef.current = new NetworkMonitor(config)
    monitorRef.current.start()

    return () => {
      monitorRef.current?.stop()
      monitorRef.current = null
    }
  }, [enabled, onQualityRecommendation])

  const recordSample = useCallback((bytes: number, durationMs: number, wasStall?: boolean) => {
    monitorRef.current?.recordSample(bytes, durationMs, wasStall)
  }, [])

  const recordStall = useCallback(() => {
    monitorRef.current?.recordStall()
  }, [])

  return {
    stats,
    recommendedQuality,
    recordSample,
    recordStall,
    isMonitoring: !!monitorRef.current
  }
}
```

**Integration with HLS.js**:

```typescript
// In VideoPlayer.tsx, integrate with HLS events:

// On fragment loaded - record network sample
hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
  const { frag, networkDetails } = data
  if (networkDetails) {
    const bytes = frag.stats?.total ?? 0
    const duration = frag.stats?.loading?.end - frag.stats?.loading?.start ?? 0
    networkMonitor.recordSample(bytes, duration)
  }
})

// On buffer stall - record stall event
hls.on(Hls.Events.ERROR, (_event, data) => {
  if (data.details === Hls.ErrorDetails.BUFFER_STALLED_ERROR) {
    networkMonitor.recordStall()
  }
})
```

**Testing**:
- Unit tests for NetworkMonitor class
- Test sample collection and pruning
- Test statistics calculation (average, stability, trend)
- Test quality recommendations at various throughput levels
- Test stall detection and handling
- Integration tests with mock HLS.js events

---

### Task 2.2: Auto Quality Mode (24 hours)

**Location**: `web/src/lib/streaming/AutoQualityController.ts`

**Purpose**: Coordinate automatic quality switching during playback based on network conditions, buffer health, and user preferences. Works alongside HLS.js's native ABR while providing additional intelligence.

**Implementation**:

```typescript
// web/src/lib/streaming/AutoQualityController.ts

import type { NetworkStats } from '../network/NetworkMonitor'
import type { VideoQualityPreferences } from '@/components/media/VideoPlayer/QualityPreferences'

export interface QualityLevel {
  index: number
  height: number
  bitrate: number
  name: string
}

export interface AutoQualityConfig {
  // Buffer thresholds (seconds)
  minBufferForUpgrade: number      // Need this much buffer to consider upgrading (default: 15)
  minBufferForDowngrade: number    // Below this, consider downgrading (default: 5)
  criticalBuffer: number           // Emergency downgrade threshold (default: 2)

  // Quality change behavior
  upgradeDelayMs: number           // Wait this long before upgrading (default: 10000)
  downgradeDelayMs: number         // Wait this long before downgrading (default: 3000)
  emergencyDowngradeDelayMs: number // Critical situations (default: 500)

  // Stability requirements
  minStabilityForUpgrade: number   // Network stability threshold (default: 0.6)
  maxStallsBeforeDowngrade: number // Stall count threshold (default: 2)
}

export interface QualityDecision {
  action: 'upgrade' | 'downgrade' | 'maintain'
  targetLevel: QualityLevel | null
  reason: string
  confidence: number // 0-1
}

export class AutoQualityController {
  private config: AutoQualityConfig
  private qualityLevels: QualityLevel[] = []
  private currentLevelIndex: number = -1
  private lastChangeTime: number = 0
  private pendingDecision: QualityDecision | null = null
  private preferences: VideoQualityPreferences | null = null

  constructor(config: Partial<AutoQualityConfig> = {}) {
    this.config = {
      minBufferForUpgrade: 15,
      minBufferForDowngrade: 5,
      criticalBuffer: 2,
      upgradeDelayMs: 10000,
      downgradeDelayMs: 3000,
      emergencyDowngradeDelayMs: 500,
      minStabilityForUpgrade: 0.6,
      maxStallsBeforeDowngrade: 2,
      ...config
    }
  }

  // Set available quality levels from HLS.js
  setQualityLevels(levels: QualityLevel[]): void {
    this.qualityLevels = levels.sort((a, b) => a.bitrate - b.bitrate)
  }

  // Update current quality level
  setCurrentLevel(index: number): void {
    this.currentLevelIndex = index
  }

  // Set user preferences
  setPreferences(prefs: VideoQualityPreferences): void {
    this.preferences = prefs
  }

  // Main decision function - call periodically during playback
  evaluate(
    networkStats: NetworkStats,
    bufferLength: number,
    isPlaying: boolean
  ): QualityDecision {
    if (!isPlaying || this.qualityLevels.length === 0) {
      return { action: 'maintain', targetLevel: null, reason: 'Not playing', confidence: 1 }
    }

    const currentLevel = this.qualityLevels[this.currentLevelIndex]
    if (!currentLevel) {
      return { action: 'maintain', targetLevel: null, reason: 'No current level', confidence: 1 }
    }

    // Check for emergency downgrade (critical buffer)
    if (bufferLength < this.config.criticalBuffer) {
      const lowestLevel = this.qualityLevels[0]
      if (lowestLevel && lowestLevel.index !== this.currentLevelIndex) {
        return {
          action: 'downgrade',
          targetLevel: lowestLevel,
          reason: `Critical buffer: ${bufferLength.toFixed(1)}s`,
          confidence: 1
        }
      }
    }

    // Check for stall-based downgrade
    if (networkStats.stallCount > this.config.maxStallsBeforeDowngrade) {
      const targetLevel = this.findLowerLevel()
      if (targetLevel) {
        return {
          action: 'downgrade',
          targetLevel,
          reason: `Too many stalls: ${networkStats.stallCount}`,
          confidence: 0.9
        }
      }
    }

    // Check for low buffer downgrade
    if (bufferLength < this.config.minBufferForDowngrade) {
      const targetLevel = this.findLowerLevel()
      if (targetLevel) {
        return {
          action: 'downgrade',
          targetLevel,
          reason: `Low buffer: ${bufferLength.toFixed(1)}s`,
          confidence: 0.8
        }
      }
    }

    // Check for network degradation
    if (networkStats.trend === 'degrading') {
      const safeBitrate = networkStats.minThroughputMbps * 1_000_000 * 0.7
      const targetLevel = this.findLevelForBitrate(safeBitrate)
      if (targetLevel && targetLevel.index < this.currentLevelIndex) {
        return {
          action: 'downgrade',
          targetLevel,
          reason: 'Network degrading',
          confidence: 0.7
        }
      }
    }

    // Check for upgrade opportunity
    const timeSinceLastChange = Date.now() - this.lastChangeTime
    if (
      bufferLength >= this.config.minBufferForUpgrade &&
      networkStats.stability >= this.config.minStabilityForUpgrade &&
      networkStats.trend !== 'degrading' &&
      timeSinceLastChange >= this.config.upgradeDelayMs
    ) {
      const safeBitrate = networkStats.averageThroughputMbps * 1_000_000 * 0.7
      const targetLevel = this.findLevelForBitrate(safeBitrate)

      // Apply user preferences
      const maxLevel = this.getMaxAllowedLevel()

      if (targetLevel && targetLevel.index > this.currentLevelIndex) {
        // Don't exceed max allowed level
        if (maxLevel && targetLevel.index > maxLevel.index) {
          if (maxLevel.index > this.currentLevelIndex) {
            return {
              action: 'upgrade',
              targetLevel: maxLevel,
              reason: 'Network supports higher quality (limited by preference)',
              confidence: 0.8
            }
          }
        } else {
          return {
            action: 'upgrade',
            targetLevel,
            reason: `Network supports ${(safeBitrate / 1_000_000).toFixed(1)} Mbps`,
            confidence: networkStats.stability
          }
        }
      }
    }

    return {
      action: 'maintain',
      targetLevel: currentLevel,
      reason: 'Conditions stable',
      confidence: 0.9
    }
  }

  // Apply a quality decision (with debouncing)
  applyDecision(decision: QualityDecision, hlsInstance: any): boolean {
    if (decision.action === 'maintain' || !decision.targetLevel) {
      return false
    }

    const delay = decision.action === 'downgrade'
      ? (decision.reason.includes('Critical') ? this.config.emergencyDowngradeDelayMs : this.config.downgradeDelayMs)
      : this.config.upgradeDelayMs

    const timeSinceLastChange = Date.now() - this.lastChangeTime
    if (timeSinceLastChange < delay) {
      return false
    }

    // Apply the change
    hlsInstance.currentLevel = decision.targetLevel.index
    this.currentLevelIndex = decision.targetLevel.index
    this.lastChangeTime = Date.now()

    return true
  }

  private findLowerLevel(): QualityLevel | null {
    const currentIndex = this.currentLevelIndex
    if (currentIndex > 0) {
      return this.qualityLevels[currentIndex - 1]
    }
    return null
  }

  private findLevelForBitrate(targetBitrate: number): QualityLevel | null {
    // Find highest level that fits within target bitrate
    for (let i = this.qualityLevels.length - 1; i >= 0; i--) {
      if (this.qualityLevels[i].bitrate <= targetBitrate) {
        return this.qualityLevels[i]
      }
    }
    return this.qualityLevels[0] || null
  }

  private getMaxAllowedLevel(): QualityLevel | null {
    if (!this.preferences?.maxAutoQuality) return null

    const maxHeight = this.parseQualityToHeight(this.preferences.maxAutoQuality)
    if (!maxHeight) return null

    for (let i = this.qualityLevels.length - 1; i >= 0; i--) {
      if (this.qualityLevels[i].height <= maxHeight) {
        return this.qualityLevels[i]
      }
    }
    return null
  }

  private parseQualityToHeight(quality: string): number | null {
    const match = quality.match(/(\d+)p?/i)
    if (match) {
      const value = parseInt(match[1], 10)
      // Handle "4k" as 2160
      if (quality.toLowerCase().includes('4k')) return 2160
      return value
    }
    return null
  }
}
```

**React Hook**:

```typescript
// web/src/lib/hooks/useAutoQuality.ts

import { useState, useEffect, useRef, useCallback } from 'react'
import { AutoQualityController, QualityLevel, QualityDecision } from '../streaming/AutoQualityController'
import { useNetworkMonitor } from './useNetworkMonitor'
import { loadPreferences, VideoQualityPreferences } from '@/components/media/VideoPlayer/QualityPreferences'
import Hls from 'hls.js'

export interface UseAutoQualityOptions {
  enabled?: boolean
  hlsInstance: Hls | null
  onQualityChange?: (level: QualityLevel, reason: string) => void
}

export const useAutoQuality = (options: UseAutoQualityOptions) => {
  const { enabled = true, hlsInstance, onQualityChange } = options
  const [currentDecision, setCurrentDecision] = useState<QualityDecision | null>(null)
  const [isAutoMode, setIsAutoMode] = useState(true)
  const controllerRef = useRef<AutoQualityController | null>(null)
  const evaluationIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Network monitoring
  const { stats: networkStats, recordSample, recordStall } = useNetworkMonitor({
    enabled: enabled && isAutoMode
  })

  // Initialize controller
  useEffect(() => {
    controllerRef.current = new AutoQualityController()

    // Load user preferences
    const prefs = loadPreferences()
    controllerRef.current.setPreferences(prefs)
    setIsAutoMode(prefs.autoQualityEnabled)

    return () => {
      if (evaluationIntervalRef.current) {
        clearInterval(evaluationIntervalRef.current)
      }
    }
  }, [])

  // Set up HLS.js integration
  useEffect(() => {
    if (!hlsInstance || !enabled) return

    const controller = controllerRef.current
    if (!controller) return

    // Populate quality levels from HLS
    const updateLevels = () => {
      const levels: QualityLevel[] = hlsInstance.levels.map((level, index) => ({
        index,
        height: level.height,
        bitrate: level.bitrate,
        name: `${level.height}p`
      }))
      controller.setQualityLevels(levels)
    }

    // Handle fragment loaded - record network sample
    const onFragLoaded = (_event: string, data: any) => {
      const stats = data.frag?.stats
      if (stats?.total && stats?.loading) {
        const duration = stats.loading.end - stats.loading.start
        recordSample(stats.total, duration)
      }
    }

    // Handle level switch
    const onLevelSwitched = (_event: string, data: any) => {
      controller.setCurrentLevel(data.level)
    }

    // Handle buffer stall
    const onError = (_event: string, data: any) => {
      if (data.details === Hls.ErrorDetails.BUFFER_STALLED_ERROR) {
        recordStall()
      }
    }

    hlsInstance.on(Hls.Events.MANIFEST_PARSED, updateLevels)
    hlsInstance.on(Hls.Events.FRAG_LOADED, onFragLoaded)
    hlsInstance.on(Hls.Events.LEVEL_SWITCHED, onLevelSwitched)
    hlsInstance.on(Hls.Events.ERROR, onError)

    // Start periodic evaluation
    evaluationIntervalRef.current = setInterval(() => {
      if (!isAutoMode || !networkStats) return

      const video = hlsInstance.media
      if (!video || video.paused) return

      // Get buffer length
      const buffered = video.buffered
      const bufferLength = buffered.length > 0
        ? buffered.end(buffered.length - 1) - video.currentTime
        : 0

      // Evaluate and potentially apply decision
      const decision = controller.evaluate(networkStats, bufferLength, !video.paused)
      setCurrentDecision(decision)

      if (decision.action !== 'maintain' && decision.targetLevel) {
        const applied = controller.applyDecision(decision, hlsInstance)
        if (applied) {
          onQualityChange?.(decision.targetLevel, decision.reason)
        }
      }
    }, 2000)

    return () => {
      hlsInstance.off(Hls.Events.MANIFEST_PARSED, updateLevels)
      hlsInstance.off(Hls.Events.FRAG_LOADED, onFragLoaded)
      hlsInstance.off(Hls.Events.LEVEL_SWITCHED, onLevelSwitched)
      hlsInstance.off(Hls.Events.ERROR, onError)

      if (evaluationIntervalRef.current) {
        clearInterval(evaluationIntervalRef.current)
      }
    }
  }, [hlsInstance, enabled, isAutoMode, networkStats, recordSample, recordStall, onQualityChange])

  // Toggle auto mode
  const toggleAutoMode = useCallback((enabled: boolean) => {
    setIsAutoMode(enabled)
    if (hlsInstance) {
      hlsInstance.currentLevel = enabled ? -1 : hlsInstance.currentLevel
    }
  }, [hlsInstance])

  // Manual quality selection
  const setQuality = useCallback((levelIndex: number) => {
    if (hlsInstance) {
      setIsAutoMode(false)
      hlsInstance.currentLevel = levelIndex
      controllerRef.current?.setCurrentLevel(levelIndex)
    }
  }, [hlsInstance])

  return {
    isAutoMode,
    currentDecision,
    networkStats,
    toggleAutoMode,
    setQuality
  }
}
```

**Testing**:
- Unit tests for AutoQualityController decision logic
- Test upgrade conditions (buffer + stability + network)
- Test downgrade triggers (buffer low, stalls, degrading network)
- Test emergency downgrade at critical buffer levels
- Test preference limits (max quality cap)
- Test debouncing behavior (upgrade/downgrade delays)
- Integration tests with mock HLS.js instance

---

### Task 2.3: Analytics Collection (16 hours)

**Location**:
- `web/src/lib/analytics/PlaybackAnalytics.ts` - Client-side collection
- `internal/api/handlers/analytics.go` - Backend API
- `internal/infrastructure/database/queries/*/playback_events.sql` - Database queries

**Purpose**: Collect playback quality metrics for:
1. Debugging user-reported issues
2. Identifying problematic content
3. Optimizing transcoding profiles
4. Understanding device/network distribution

**Client-Side Implementation**:

```typescript
// web/src/lib/analytics/PlaybackAnalytics.ts

export interface QualitySwitchEvent {
  mediaId: number
  sessionId: string
  fromQuality: string | null
  toQuality: string
  switchReason: 'auto_bandwidth' | 'auto_buffer' | 'auto_stall' | 'user_manual' | 'initial'
  positionSeconds: number
  networkSpeedMbps: number | null
  bufferSeconds: number | null
  causedStall: boolean
  deviceType: string
  connectionType: string
  timestamp: number
}

export interface PlaybackSession {
  sessionId: string
  mediaId: number
  startTime: number
  endTime: number | null
  totalPlayTime: number
  totalBufferTime: number
  stallCount: number
  qualitySwitchCount: number
  averageQuality: string
  deviceType: string
  connectionType: string
}

export interface PlaybackAnalyticsConfig {
  enabled: boolean
  batchSize: number           // Send events in batches (default: 10)
  flushIntervalMs: number     // Or flush every N ms (default: 30000)
  endpoint: string            // API endpoint
}

export class PlaybackAnalytics {
  private config: PlaybackAnalyticsConfig
  private eventQueue: QualitySwitchEvent[] = []
  private currentSession: PlaybackSession | null = null
  private flushTimeout: ReturnType<typeof setTimeout> | null = null

  constructor(config: Partial<PlaybackAnalyticsConfig> = {}) {
    this.config = {
      enabled: true,
      batchSize: 10,
      flushIntervalMs: 30000,
      endpoint: '/api/analytics/playback',
      ...config
    }
  }

  // Start a new playback session
  startSession(mediaId: number): string {
    const sessionId = `${mediaId}-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

    this.currentSession = {
      sessionId,
      mediaId,
      startTime: Date.now(),
      endTime: null,
      totalPlayTime: 0,
      totalBufferTime: 0,
      stallCount: 0,
      qualitySwitchCount: 0,
      averageQuality: '',
      deviceType: this.getDeviceType(),
      connectionType: this.getConnectionType()
    }

    this.scheduleFlush()
    return sessionId
  }

  // End the current session
  endSession(): void {
    if (this.currentSession) {
      this.currentSession.endTime = Date.now()
      this.flush() // Flush remaining events
    }
    this.currentSession = null
  }

  // Record a quality switch event
  recordQualitySwitch(
    fromQuality: string | null,
    toQuality: string,
    reason: QualitySwitchEvent['switchReason'],
    positionSeconds: number,
    networkSpeedMbps: number | null,
    bufferSeconds: number | null,
    causedStall: boolean = false
  ): void {
    if (!this.config.enabled || !this.currentSession) return

    const event: QualitySwitchEvent = {
      mediaId: this.currentSession.mediaId,
      sessionId: this.currentSession.sessionId,
      fromQuality,
      toQuality,
      switchReason: reason,
      positionSeconds,
      networkSpeedMbps,
      bufferSeconds,
      causedStall,
      deviceType: this.currentSession.deviceType,
      connectionType: this.getConnectionType(),
      timestamp: Date.now()
    }

    this.eventQueue.push(event)
    this.currentSession.qualitySwitchCount++

    if (this.eventQueue.length >= this.config.batchSize) {
      this.flush()
    }
  }

  // Record a stall event
  recordStall(durationMs: number): void {
    if (this.currentSession) {
      this.currentSession.stallCount++
      this.currentSession.totalBufferTime += durationMs
    }
  }

  // Record play time
  recordPlayTime(durationMs: number): void {
    if (this.currentSession) {
      this.currentSession.totalPlayTime += durationMs
    }
  }

  private scheduleFlush(): void {
    if (this.flushTimeout) {
      clearTimeout(this.flushTimeout)
    }
    this.flushTimeout = setTimeout(() => {
      this.flush()
      this.scheduleFlush()
    }, this.config.flushIntervalMs)
  }

  private async flush(): Promise<void> {
    if (this.eventQueue.length === 0) return

    const events = [...this.eventQueue]
    this.eventQueue = []

    try {
      await fetch(this.config.endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session: this.currentSession,
          events
        })
      })
    } catch (error) {
      // Re-queue events on failure (with limit)
      if (this.eventQueue.length < 100) {
        this.eventQueue.unshift(...events)
      }
    }
  }

  private getDeviceType(): string {
    const ua = navigator.userAgent
    if (/tablet|ipad/i.test(ua)) return 'tablet'
    if (/mobile|iphone|android/i.test(ua)) return 'mobile'
    if (/smart-tv|smarttv|tv/i.test(ua)) return 'tv'
    return 'desktop'
  }

  private getConnectionType(): string {
    const connection = (navigator as any).connection
    return connection?.effectiveType ?? 'unknown'
  }
}

// Singleton instance
export const playbackAnalytics = new PlaybackAnalytics()
```

**Backend API Handler**:

```go
// internal/api/handlers/analytics.go

package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

// PlaybackEventRequest represents incoming analytics data
type PlaybackEventRequest struct {
    Session *PlaybackSessionData    `json:"session"`
    Events  []QualitySwitchEventData `json:"events"`
}

type PlaybackSessionData struct {
    SessionID         string `json:"sessionId"`
    MediaID           int64  `json:"mediaId"`
    StartTime         int64  `json:"startTime"`
    EndTime           *int64 `json:"endTime"`
    TotalPlayTime     int64  `json:"totalPlayTime"`
    TotalBufferTime   int64  `json:"totalBufferTime"`
    StallCount        int    `json:"stallCount"`
    QualitySwitchCount int   `json:"qualitySwitchCount"`
    DeviceType        string `json:"deviceType"`
    ConnectionType    string `json:"connectionType"`
}

type QualitySwitchEventData struct {
    MediaID         int64   `json:"mediaId"`
    SessionID       string  `json:"sessionId"`
    FromQuality     *string `json:"fromQuality"`
    ToQuality       string  `json:"toQuality"`
    SwitchReason    string  `json:"switchReason"`
    PositionSeconds float64 `json:"positionSeconds"`
    NetworkSpeedMbps *float64 `json:"networkSpeedMbps"`
    BufferSeconds    *float64 `json:"bufferSeconds"`
    CausedStall     bool    `json:"causedStall"`
    DeviceType      string  `json:"deviceType"`
    ConnectionType  string  `json:"connectionType"`
    Timestamp       int64   `json:"timestamp"`
}

type AnalyticsHandler struct {
    repo AnalyticsRepository // Interface for database operations
}

func NewAnalyticsHandler(repo AnalyticsRepository) *AnalyticsHandler {
    return &AnalyticsHandler{repo: repo}
}

// RecordPlaybackEvents records playback quality events
// POST /api/analytics/playback
func (h *AnalyticsHandler) RecordPlaybackEvents(c *gin.Context) {
    var req PlaybackEventRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
        return
    }

    // Store events in database
    ctx := c.Request.Context()

    for _, event := range req.Events {
        if err := h.repo.SaveQualitySwitchEvent(ctx, event); err != nil {
            // Log error but don't fail the request
            // Analytics shouldn't block playback
            continue
        }
    }

    c.JSON(http.StatusAccepted, gin.H{"received": len(req.Events)})
}

// GetPlaybackStats returns aggregated playback statistics
// GET /api/analytics/stats
func (h *AnalyticsHandler) GetPlaybackStats(c *gin.Context) {
    mediaID := c.Query("media_id")
    period := c.DefaultQuery("period", "24h")

    ctx := c.Request.Context()
    stats, err := h.repo.GetAggregatedStats(ctx, mediaID, period)
    if err != nil {
        c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get stats"})
        return
    }

    c.JSON(http.StatusOK, stats)
}
```

**Database Schema** (already partially added in Phase 1 migration):

```sql
-- Query to get quality switch distribution
-- name: GetQualitySwitchDistribution :many
SELECT
    switch_reason,
    to_quality,
    COUNT(*) as count,
    AVG(CASE WHEN caused_stall THEN 1 ELSE 0 END) as stall_rate
FROM playback_quality_events
WHERE created_at > datetime('now', '-24 hours')
GROUP BY switch_reason, to_quality
ORDER BY count DESC;

-- Query to get device/connection breakdown
-- name: GetDeviceConnectionBreakdown :many
SELECT
    device_type,
    connection_type,
    COUNT(DISTINCT session_id) as session_count,
    AVG(network_speed_mbps) as avg_speed
FROM playback_quality_events
WHERE created_at > datetime('now', '-7 days')
GROUP BY device_type, connection_type;
```

**Testing**:
- Unit tests for PlaybackAnalytics class
- Test event batching and flush behavior
- Test session lifecycle (start/end)
- Backend API tests with mock data
- Database query tests
- Integration test: full event flow from client to database

---

### Task 2.4: Enhanced Player Controls (20 hours)

**Location**:
- `web/src/components/media/VideoPlayer/VideoControls.tsx` - Update existing
- `web/src/components/media/VideoPlayer/NetworkOverlay.tsx` - New component
- `web/src/components/media/VideoPlayer/BufferIndicator.tsx` - New component

**Purpose**: Provide users with visibility into adaptive streaming behavior and manual override controls.

**NetworkOverlay Component**:

```typescript
// web/src/components/media/VideoPlayer/NetworkOverlay.tsx

import type { NetworkStats } from '@/lib/network/NetworkMonitor'
import type { QualityDecision } from '@/lib/streaming/AutoQualityController'

interface NetworkOverlayProps {
  stats: NetworkStats | null
  decision: QualityDecision | null
  currentQuality: string
  isVisible: boolean
}

export const NetworkOverlay = ({
  stats,
  decision,
  currentQuality,
  isVisible
}: NetworkOverlayProps) => {
  if (!isVisible || !stats) return null

  const getTrendIcon = () => {
    switch (stats.trend) {
      case 'improving': return '↑'
      case 'degrading': return '↓'
      default: return '→'
    }
  }

  const getTrendColor = () => {
    switch (stats.trend) {
      case 'improving': return 'text-green-400'
      case 'degrading': return 'text-red-400'
      default: return 'text-white/70'
    }
  }

  return (
    <div className="absolute top-16 left-4 z-30 bg-black/80 backdrop-blur-sm rounded-lg p-3 text-xs font-mono text-white/90 space-y-1 min-w-48">
      {/* Network speed */}
      <div className="flex justify-between">
        <span className="text-white/60">Speed:</span>
        <span className={getTrendColor()}>
          {stats.averageThroughputMbps.toFixed(1)} Mbps {getTrendIcon()}
        </span>
      </div>

      {/* Stability */}
      <div className="flex justify-between">
        <span className="text-white/60">Stability:</span>
        <span className={stats.stability > 0.7 ? 'text-green-400' : stats.stability > 0.4 ? 'text-yellow-400' : 'text-red-400'}>
          {(stats.stability * 100).toFixed(0)}%
        </span>
      </div>

      {/* Current quality */}
      <div className="flex justify-between">
        <span className="text-white/60">Quality:</span>
        <span>{currentQuality}</span>
      </div>

      {/* Connection type */}
      <div className="flex justify-between">
        <span className="text-white/60">Connection:</span>
        <span>{stats.connectionType}</span>
      </div>

      {/* Stalls */}
      {stats.stallCount > 0 && (
        <div className="flex justify-between text-red-400">
          <span>Stalls:</span>
          <span>{stats.stallCount}</span>
        </div>
      )}

      {/* Decision */}
      {decision && decision.action !== 'maintain' && (
        <div className="mt-2 pt-2 border-t border-white/20">
          <div className="text-yellow-400">
            {decision.action === 'upgrade' ? '↑ Upgrading' : '↓ Downgrading'}
          </div>
          <div className="text-white/50 text-[10px]">{decision.reason}</div>
        </div>
      )}
    </div>
  )
}
```

**BufferIndicator Component**:

```typescript
// web/src/components/media/VideoPlayer/BufferIndicator.tsx

interface BufferIndicatorProps {
  bufferLength: number      // Seconds of buffered content ahead
  minBuffer: number         // Threshold for warning (default: 5)
  criticalBuffer: number    // Threshold for critical (default: 2)
  isVisible: boolean
}

export const BufferIndicator = ({
  bufferLength,
  minBuffer = 5,
  criticalBuffer = 2,
  isVisible
}: BufferIndicatorProps) => {
  if (!isVisible) return null

  const getBufferColor = () => {
    if (bufferLength < criticalBuffer) return 'bg-red-500'
    if (bufferLength < minBuffer) return 'bg-yellow-500'
    return 'bg-green-500'
  }

  const getBufferWidth = () => {
    // Show buffer as percentage of 30 seconds (target buffer)
    const percentage = Math.min(100, (bufferLength / 30) * 100)
    return `${percentage}%`
  }

  return (
    <div className="absolute bottom-24 left-4 right-4 z-20">
      {/* Buffer bar (thin line above main progress bar) */}
      <div className="h-0.5 bg-white/20 rounded-full overflow-hidden">
        <div
          className={`h-full ${getBufferColor()} transition-all duration-300`}
          style={{ width: getBufferWidth() }}
        />
      </div>

      {/* Buffer time label (only show when low) */}
      {bufferLength < minBuffer && (
        <div className={`absolute -top-5 left-0 text-xs ${
          bufferLength < criticalBuffer ? 'text-red-400' : 'text-yellow-400'
        }`}>
          Buffer: {bufferLength.toFixed(1)}s
        </div>
      )}
    </div>
  )
}
```

**Updated VideoControls Integration**:

```typescript
// Additions to VideoControls.tsx

interface VideoControlsProps {
  // ... existing props ...

  // New props for adaptive streaming
  networkStats?: NetworkStats | null
  autoQualityDecision?: QualityDecision | null
  isAutoMode?: boolean
  bufferLength?: number
  onAutoModeToggle?: (enabled: boolean) => void
  showDebugOverlay?: boolean
  onDebugToggleChange?: (show: boolean) => void
}

// In the component, add debug toggle button:
<button
  onClick={() => onDebugToggleChange?.(!showDebugOverlay)}
  className={`hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer ${
    showDebugOverlay ? 'bg-white/30' : ''
  }`}
  aria-label="Toggle network debug overlay"
  title="Show network stats (D)"
>
  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
    <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clipRule="evenodd" />
  </svg>
</button>

// Add keyboard shortcut 'D' for debug overlay toggle
case 'd':
case 'D':
  e.preventDefault()
  onDebugToggleChange?.(!showDebugOverlay)
  break
```

**Testing**:
- Visual regression tests for new components
- Test NetworkOverlay rendering with various stats
- Test BufferIndicator color transitions
- Test keyboard shortcut integration
- Test debug overlay toggle behavior
- Accessibility tests (screen reader announcements)

---

### Task 2.5: Testing & Optimization (20 hours)

**Purpose**: Comprehensive testing of the adaptive streaming system and performance optimization.

**Test Categories**:

1. **Unit Tests** (8 hours)
   - NetworkMonitor statistics calculation
   - AutoQualityController decision logic
   - PlaybackAnalytics event batching
   - Edge cases (empty data, null values, boundary conditions)

2. **Integration Tests** (6 hours)
   - Full playback flow with quality switches
   - Network simulation (throttling, disconnects)
   - Analytics event delivery
   - HLS.js event integration

3. **Performance Tests** (4 hours)
   - Memory usage during long playback sessions
   - CPU usage of monitoring/analytics
   - Network overhead of analytics events
   - React re-render optimization

4. **Manual Testing Scenarios** (2 hours)
   - Test on various devices (desktop, mobile, tablet)
   - Test on different networks (WiFi, 4G, 3G simulation)
   - Test with various content types (SD, HD, 4K)
   - Test preference combinations

**Network Simulation Test Setup**:

```typescript
// web/src/lib/network/__tests__/NetworkMonitor.test.ts

import { NetworkMonitor, NetworkStats } from '../NetworkMonitor'

describe('NetworkMonitor', () => {
  let monitor: NetworkMonitor

  beforeEach(() => {
    monitor = new NetworkMonitor({
      sampleWindowMs: 10000,
      minSamplesForStats: 3
    })
  })

  afterEach(() => {
    monitor.stop()
  })

  describe('statistics calculation', () => {
    it('calculates average throughput correctly', () => {
      monitor.start()

      // Simulate 3 samples: 1MB in 1s each = 8 Mbps
      monitor.recordSample(1_000_000, 1000) // 8 Mbps
      monitor.recordSample(1_000_000, 1000) // 8 Mbps
      monitor.recordSample(1_000_000, 1000) // 8 Mbps

      const stats = monitor.getStats()
      expect(stats?.averageThroughputMbps).toBeCloseTo(8, 1)
    })

    it('detects degrading network trend', () => {
      monitor.start()

      // First half: fast
      monitor.recordSample(2_000_000, 1000) // 16 Mbps
      monitor.recordSample(2_000_000, 1000)

      // Second half: slow
      monitor.recordSample(500_000, 1000) // 4 Mbps
      monitor.recordSample(500_000, 1000)

      const stats = monitor.getStats()
      expect(stats?.trend).toBe('degrading')
    })

    it('calculates stability correctly', () => {
      monitor.start()

      // Stable: consistent throughput
      for (let i = 0; i < 5; i++) {
        monitor.recordSample(1_000_000, 1000) // 8 Mbps each
      }

      const stats = monitor.getStats()
      expect(stats?.stability).toBeGreaterThan(0.9)
    })

    it('detects low stability with variable throughput', () => {
      monitor.start()

      // Unstable: varying throughput
      monitor.recordSample(500_000, 1000)  // 4 Mbps
      monitor.recordSample(2_000_000, 1000) // 16 Mbps
      monitor.recordSample(250_000, 1000)  // 2 Mbps
      monitor.recordSample(1_500_000, 1000) // 12 Mbps

      const stats = monitor.getStats()
      expect(stats?.stability).toBeLessThan(0.5)
    })

    it('counts stalls correctly', () => {
      monitor.start()

      monitor.recordSample(1_000_000, 1000, true) // stall
      monitor.recordSample(1_000_000, 1000, false)
      monitor.recordSample(1_000_000, 1000, true) // stall

      const stats = monitor.getStats()
      expect(stats?.stallCount).toBe(2)
    })
  })
})
```

**AutoQualityController Tests**:

```typescript
// web/src/lib/streaming/__tests__/AutoQualityController.test.ts

import { AutoQualityController, QualityLevel } from '../AutoQualityController'
import { NetworkStats } from '../../network/NetworkMonitor'

describe('AutoQualityController', () => {
  let controller: AutoQualityController
  const mockLevels: QualityLevel[] = [
    { index: 0, height: 360, bitrate: 800_000, name: '360p' },
    { index: 1, height: 480, bitrate: 1_800_000, name: '480p' },
    { index: 2, height: 720, bitrate: 4_000_000, name: '720p' },
    { index: 3, height: 1080, bitrate: 8_000_000, name: '1080p' },
  ]

  beforeEach(() => {
    controller = new AutoQualityController({
      minBufferForUpgrade: 15,
      minBufferForDowngrade: 5,
      criticalBuffer: 2,
      upgradeDelayMs: 0, // Disable delay for testing
      downgradeDelayMs: 0
    })
    controller.setQualityLevels(mockLevels)
    controller.setCurrentLevel(2) // Start at 720p
  })

  describe('downgrade decisions', () => {
    it('triggers emergency downgrade at critical buffer', () => {
      const stats = createMockStats({ averageThroughputMbps: 10, stability: 0.8 })
      const decision = controller.evaluate(stats, 1.5, true) // Buffer < 2s

      expect(decision.action).toBe('downgrade')
      expect(decision.targetLevel?.height).toBe(360) // Lowest
      expect(decision.reason).toContain('Critical buffer')
    })

    it('triggers downgrade on excessive stalls', () => {
      const stats = createMockStats({ stallCount: 3 })
      const decision = controller.evaluate(stats, 10, true)

      expect(decision.action).toBe('downgrade')
      expect(decision.reason).toContain('stalls')
    })

    it('triggers downgrade on low buffer', () => {
      const stats = createMockStats({ averageThroughputMbps: 5 })
      const decision = controller.evaluate(stats, 3, true) // Buffer < 5s

      expect(decision.action).toBe('downgrade')
      expect(decision.reason).toContain('Low buffer')
    })
  })

  describe('upgrade decisions', () => {
    it('upgrades when network and buffer support it', () => {
      controller.setCurrentLevel(1) // 480p
      const stats = createMockStats({
        averageThroughputMbps: 15,
        stability: 0.8,
        trend: 'stable'
      })

      const decision = controller.evaluate(stats, 20, true) // Good buffer

      expect(decision.action).toBe('upgrade')
      expect(decision.targetLevel?.height).toBeGreaterThan(480)
    })

    it('does not upgrade with degrading network', () => {
      controller.setCurrentLevel(1)
      const stats = createMockStats({
        averageThroughputMbps: 15,
        stability: 0.8,
        trend: 'degrading'
      })

      const decision = controller.evaluate(stats, 20, true)

      expect(decision.action).not.toBe('upgrade')
    })

    it('does not upgrade with low stability', () => {
      controller.setCurrentLevel(1)
      const stats = createMockStats({
        averageThroughputMbps: 15,
        stability: 0.3, // Low stability
        trend: 'stable'
      })

      const decision = controller.evaluate(stats, 20, true)

      expect(decision.action).not.toBe('upgrade')
    })
  })

  describe('preference limits', () => {
    it('respects max quality preference', () => {
      controller.setCurrentLevel(0) // 360p
      controller.setPreferences({
        autoQualityEnabled: true,
        maxAutoQuality: '720p',
        // ... other prefs
      })

      const stats = createMockStats({ averageThroughputMbps: 50 })
      const decision = controller.evaluate(stats, 20, true)

      // Should not recommend 1080p even though network supports it
      if (decision.action === 'upgrade' && decision.targetLevel) {
        expect(decision.targetLevel.height).toBeLessThanOrEqual(720)
      }
    })
  })
})

function createMockStats(overrides: Partial<NetworkStats> = {}): NetworkStats {
  return {
    currentThroughputMbps: 10,
    averageThroughputMbps: 10,
    minThroughputMbps: 8,
    maxThroughputMbps: 12,
    stability: 0.8,
    stallCount: 0,
    lastStallTime: null,
    trend: 'stable',
    connectionType: 'wifi',
    isMetered: false,
    ...overrides
  }
}
```

**Performance Optimization Checklist**:

- [ ] NetworkMonitor: Use requestAnimationFrame instead of setInterval for stats updates
- [ ] Analytics: Implement exponential backoff for failed event submissions
- [ ] React: Memoize NetworkOverlay and BufferIndicator components
- [ ] React: Use useCallback for all event handlers passed to child components
- [ ] HLS.js: Tune buffer configuration based on device type
- [ ] Memory: Implement sample pruning in NetworkMonitor to prevent unbounded growth
- [ ] CPU: Throttle stats calculations during background tab

---

### Phase 2 Completion Checklist

Before proceeding to Phase 3, verify:

**Core Functionality**:
- [x] NetworkMonitor collects accurate throughput samples from HLS.js
- [x] NetworkMonitor detects stalls and reports them correctly
- [x] AutoQualityController makes appropriate upgrade/downgrade decisions
- [ ] Quality switches are smooth (no visible stuttering) - *needs manual testing*
- [x] User preferences are respected (max quality limits, cellular restrictions)

**Analytics**:
- [x] PlaybackAnalytics batches and sends events reliably
- [x] Backend stores events in database (migration 000021, SQLC queries, repository)
- [x] Events survive page navigation (sendBeacon fallback)
- [ ] Analytics overhead is minimal (<1% CPU, <100KB/hour network) - *needs profiling*

**UI/UX**:
- [x] NetworkOverlay displays accurate real-time stats (draggable, with throughput graph)
- [x] BufferIndicator shows buffer health (removed per user preference - info in NetworkOverlay)
- [x] Debug overlay toggle works (keyboard shortcut 'D', NerdMenu gear icon)
- [x] Quality selector shows auto mode status
- [ ] Smooth transitions between quality levels - *needs manual testing*

**Testing**:
- [ ] All unit tests pass - *needs test suite run*
- [ ] Integration tests pass - *needs test suite run*
- [ ] Manual testing on desktop, mobile, tablet completed
- [ ] Network throttling scenarios tested
- [ ] Memory usage stable during 2+ hour playback

**Documentation**:
- [x] API endpoints documented in OpenAPI spec (Swagger annotations on handler)
- [x] Analytics event schema documented (types in handler and repository)
- [ ] User-facing help text for quality settings

---

## Phase 3: Multi-Codec Support (Week 5-6)
**Duration**: 80-100 hours
**Goal**: Support modern codecs for bandwidth savings

[Detailed breakdown similar to Phase 1...]

---

## Phase 4: Legacy System Migration (Week 7)

**Duration**: 32-40 hours
**Goal**: Deprecate and remove legacy DASH-based transcoding system

**Background**: The codebase currently contains two parallel transcoding systems:

- **Legacy**: `QualityProfile` in `profiles.go` with 4 basic profiles (360p, 720p, 1080p, 4K) and string-based bitrates
- **New**: `AdaptiveProfile` in `adaptive_profiles.go` with 34 granular profiles and comprehensive metadata

This phase consolidates to a single HLS-based system using the new adaptive profiles.

### Task 4.1: Create Backward Compatibility Layer (8 hours)

**Location**: `internal/infrastructure/transcoding/adaptive_profiles.go`

**Implementation**:

```go
// GetProfileForLegacyQuality maps legacy quality strings (360p, 720p, 1080p, 4k) to AdaptiveProfile IDs.
// This maintains backward compatibility with the domain layer's quality constants while using
// the new granular AdaptiveProfile system internally.
func GetProfileForLegacyQuality(quality string) (*AdaptiveProfile, error) {
    // Map legacy quality strings to specific AdaptiveProfile IDs
    var profileID string
    switch quality {
    case transcode.Quality360p:
        profileID = Quality360p800k // 360p @ 800 kbps
    case transcode.Quality720p:
        profileID = Quality720p2500k // 720p @ 2.5 Mbps (standard)
    case transcode.Quality1080p:
        profileID = Quality1080p5000k // 1080p @ 5 Mbps (standard)
    case transcode.Quality4K:
        profileID = Quality4K15000k // 4K @ 15 Mbps (standard)
    default:
        return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
    }

    return GetAdaptiveProfile(profileID)
}
```

**Testing**:

- Unit tests for legacy quality mapping
- Verify each legacy quality maps to correct adaptive profile
- Test error handling for invalid quality strings

### Task 4.2: Migrate Transcoding Infrastructure (16 hours)

**Files to Update**:

1. `internal/infrastructure/transcoding/job_executor.go` (4 hours)
   - Replace `QualityProfile` with `AdaptiveProfile`
   - Use `GetProfileForLegacyQuality()` for domain layer queries
   - Convert string bitrates to integer bitrates

2. `internal/infrastructure/transcoding/ffmpeg_args_builder.go` (4 hours)
   - Update to accept `AdaptiveProfile` instead of `QualityProfile`
   - Use integer bitrates directly (no string parsing needed)
   - Leverage additional profile metadata (preset, CRF, etc.)

3. `internal/infrastructure/transcoding/session_manager.go` (4 hours)
   - Update session storage to use `AdaptiveProfile`
   - Maintain backward compatibility for existing sessions
   - Add migration logic for in-progress jobs

4. `internal/application/transcode/serve_manifest.go` (4 hours)
   - Use `GetProfileForLegacyQuality()` when serving manifests
   - Ensure existing HLS URLs continue to work
   - Update quality validation logic

**Testing**:

- Integration tests for each updated component
- Test all streaming strategies with new profiles
- Verify existing transcode jobs continue working

### Task 4.3: Remove Legacy Code (4 hours)

**Files to Delete**:

- `internal/infrastructure/transcoding/profiles.go` (entire file)

**Functions to Remove**:

- `GetQualityProfile(quality string) (*QualityProfile, error)`
- `GetAllProfiles() []*QualityProfile`
- `IsQualitySupported(quality string) bool`
- `GetSupportedQualities() []string`

**Cleanup**:

- Remove all references to `QualityProfile` type
- Update imports to remove profiles.go references
- Clean up any helper functions that are no longer needed

### Task 4.4: Update Domain Layer (4 hours)

**Location**: `internal/domain/transcode/`

**Changes**:

- **Keep** existing quality constants unchanged:
  - `Quality360p = "360p"`
  - `Quality720p = "720p"`
  - `Quality1080p = "1080p"`
  - `Quality4K = "4k"`
- These constants remain the public API contract
- Infrastructure layer handles mapping to granular profiles
- `isValidQuality()` continues to validate these 4 levels

**Documentation**:

- Update godoc comments to clarify domain/infrastructure separation
- Document that infrastructure uses AdaptiveProfile internally
- Add migration guide for any external callers

### Task 4.5: Comprehensive Testing (8 hours)

**Test Scenarios**:

1. **Existing Functionality** (3 hours)
   - Start new transcode job with each legacy quality (360p, 720p, 1080p, 4K)
   - Verify all four streaming strategies still work (DirectPlay, Remux, RemuxWithAudio, Transcode)
   - Test session persistence and resumption
   - Validate HLS manifest generation

2. **Edge Cases** (2 hours)
   - In-progress jobs from old system
   - Invalid quality strings
   - Missing profile IDs
   - Concurrent transcode requests

3. **Performance Validation** (2 hours)
   - Measure transcode time before/after migration
   - Check memory usage with new profiles
   - Validate CPU usage remains stable
   - Compare output quality metrics

4. **Integration Testing** (1 hour)
   - End-to-end playback test for each quality
   - Quality switching during playback
   - Seeking within transcoded streams

**Success Metrics**:

- All tests pass
- Zero increase in transcode failure rate
- No performance regression (< 5% variance)
- Playback quality metrics unchanged

### Phase 4 Completion Checklist

- [ ] `GetProfileForLegacyQuality()` function implemented and tested
- [ ] `job_executor.go` migrated to AdaptiveProfile
- [ ] `ffmpeg_args_builder.go` migrated to AdaptiveProfile
- [ ] `session_manager.go` migrated to AdaptiveProfile
- [ ] `serve_manifest.go` migrated to AdaptiveProfile
- [ ] `profiles.go` file deleted
- [ ] All legacy `QualityProfile` references removed
- [ ] Domain layer constants unchanged and validated
- [ ] All existing transcode jobs work with new system
- [ ] All streaming strategies tested and functional
- [ ] Performance metrics show no regressions
- [ ] Integration tests passing
- [ ] Migration documentation complete

**Phase 4 Deliverables**:

- ✅ Single unified HLS-based transcoding system
- ✅ All legacy DASH code removed
- ✅ Backward compatibility maintained at API level
- ✅ Zero functional regressions
- ✅ Cleaner codebase with less duplication
- ✅ Clear domain/infrastructure separation

---

## Phase 5: Advanced Features (Week 8-9)
**Duration**: 80-100 hours
**Goal**: Polish, optimization, and future-proofing

[Detailed breakdown...]

---

## Risk Management

### Technical Risks

1. **Browser Capability API Support**
   - Risk: Older browsers may not support Media Capabilities API
   - Mitigation: Graceful degradation, feature detection
   - Fallback: Use basic codec detection via canPlayType()

2. **Network Speed Measurement Accuracy**
   - Risk: Single test may not represent sustained speed
   - Mitigation: Multiple measurements, weighted average
   - Fallback: Use connection API estimates

3. **FFmpeg Encoding Performance**
   - Risk: Multi-codec support may slow encoding
   - Mitigation: Hardware acceleration, profile optimization
   - Fallback: Async encoding queue, user notification

### Project Risks

1. **Scope Creep**
   - Risk: Additional features requested during development
   - Mitigation: Strict phase boundaries, deferred feature list
   - Response: Evaluate impact, defer to Phase 4+ if non-critical

2. **Timeline Overrun**
   - Risk: Complex features take longer than estimated
   - Mitigation: Weekly progress reviews, buffer time in estimates
   - Response: Deprioritize Phase 4 features if needed

3. **Testing Complexity**
   - Risk: Many device/network/browser combinations to test
   - Mitigation: Automated testing, cloud device farms
   - Response: Focus on top 10 configurations first

---

## Success Criteria

### Phase 1 Success Criteria
- [ ] Recommendation accuracy > 70% (users accept recommendation)
- [ ] Capability detection < 2 seconds
- [ ] Recommendation API response < 500ms
- [ ] Zero regressions in existing playback
- [ ] All unit tests passing
- [ ] Documentation complete

### Phase 2 Success Criteria
- [ ] Buffer rate < 1% of playback time
- [ ] Quality switches < 3 per session
- [ ] Auto mode adoption > 60%
- [ ] Network monitoring overhead < 1% CPU

### Phase 3 Success Criteria
- [ ] Bandwidth savings 30-40% with H.265/VP9
- [ ] Codec detection accuracy > 95%
- [ ] Hardware acceleration usage > 70% where available
- [ ] Zero playback failures from codec issues

### Phase 4 Success Criteria

- [ ] All transcode jobs complete successfully with new profiles
- [ ] No increase in failure rates
- [ ] Code complexity reduced (fewer lines of code)
- [ ] Clear separation between domain constants and infrastructure profiles

### Phase 5 Success Criteria

- [ ] AV1 support functional for Chrome 90+
- [ ] Analytics dashboard operational
- [ ] A/B testing framework ready
- [ ] Documentation completeness > 90%

---

## Timeline

```
Week 1-2: Phase 1 - Foundation
├─ Week 1: Capability detection + Backend profiles
└─ Week 2: Recommendation engine + Frontend integration

Week 3-4: Phase 2 - Adaptive Streaming
├─ Week 3: Network monitoring + Auto mode
└─ Week 4: Analytics + Enhanced controls

Week 5-6: Phase 3 - Multi-Codec Support
├─ Week 5: H.265 + VP9 implementation
└─ Week 6: Testing + Optimization

Week 7: Phase 4 - Legacy System Migration
└─ Week 7: Migrate to unified HLS system, remove DASH code

Week 8-9: Phase 5 - Advanced Features
├─ Week 8: AV1 + Analytics dashboard
└─ Week 9: A/B testing + Documentation
```

---

## Resource Requirements

### Development Resources
- **Backend Developer**: Full-time, Weeks 1-6
- **Frontend Developer**: Full-time, Weeks 1-8
- **QA Engineer**: Part-time (20 hrs/week), Weeks 2-8

### Infrastructure
- **Test Devices**:
  - iPhone (Safari testing)
  - Android device (Chrome/Firefox testing)
  - Desktop browsers (Chrome, Firefox, Safari, Edge)

- **Cloud Resources**:
  - Increased storage for multiple codec transcodes (estimate: +50% storage)
  - Additional CPU for multi-codec encoding (estimate: +30% compute)

- **Monitoring**:
  - Analytics database (PostgreSQL or TimeSeries DB)
  - Grafana dashboard for metrics

### External Dependencies
- FFmpeg with H.265, VP9, AV1 codecs compiled
- Hardware acceleration drivers (NVENC, QSV, VideoToolbox)
- Browser testing services (BrowserStack or similar)

---

## Post-Launch Activities

### Week 9: Monitoring & Tuning
- Monitor recommendation accuracy
- Analyze user override patterns
- Tune scoring algorithm based on data
- Address any critical bugs

### Week 10: Optimization
- Profile performance bottlenecks
- Optimize database queries
- Reduce API latency
- Improve cache hit rates

### Week 11-12: Future Planning
- Gather user feedback
- Plan Phase 5 enhancements
- Document lessons learned
- Update architecture diagrams

---

## Appendix

### A. API Endpoint Reference

**Quality Recommendation**:

- `POST /api/media/:id/recommend-quality` - Get optimal quality recommendation based on client capabilities

**Speed Testing**:

- `GET /api/speedtest/chunk` - 500KB test chunk for network speed measurement (rate-limited)

**User Preferences**:

- `PUT /api/users/:id/video-preferences` - Update video quality preferences

**Analytics**:

- `POST /api/playback/quality-switch` - Track quality switch events for analytics

**HLS Streaming** (existing, updated):

- `GET /api/media/:id/hls/:quality/playlist.m3u8` - HLS playlist for specific quality
  - Supports `?start=X` query parameter for seeking to specific timestamp
  - Quality format: `{resolution}-{bitrate}` (e.g., `1080p-8000k`, `4k-40000k`)
  - **Note**: Codec selection handled via quality profile ID, no `?codec=` parameter needed
  - **Note**: Session tracking handled internally, no `?client_id=` parameter needed

**Direct Playback & Container Remuxing**:

- System automatically detects when direct playback is possible (H.264 + AAC stereo + MP4/WebM)
- Container remuxing (MKV → HLS) handled automatically when video/audio are compatible
- Four processing tiers:
  1. **DirectPlay** (instant): No processing needed
  2. **Remux** (2-5 min): Container conversion only
  3. **RemuxWithAudioDownmix** (5-10 min): Video copy + audio transcode
  4. **Transcode** (20-60 min): Full re-encode

### B. Database Schema
- `transcode_jobs` (extended)
- `user_video_preferences`
- `quality_switches`
- `playback_metrics`

### C. Configuration Files
- `web/src/lib/capabilities/` (all files)
- `internal/infrastructure/transcoding/profiles.go`
- `internal/application/transcode/recommend_quality.go`

### D. Testing Checklist
- [ ] Unit tests
- [ ] Integration tests
- [ ] Browser compatibility tests
- [ ] Performance tests
- [ ] User acceptance tests

---

**Document Version**: 1.0
**Last Updated**: 2025-11-24
**Next Review**: End of Phase 1
