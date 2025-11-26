// Codec capability with detailed information for quality/performance tradeoffs
export interface CodecCapability {
  codec: 'h264' | 'h265' | 'vp9' | 'av1' | 'vp8'
  supported: boolean
  hardwareAccelerated: boolean
  powerEfficient: boolean
  smooth: boolean
  // Maximum resolution/fps this codec can handle smoothly
  maxWidth: number
  maxHeight: number
  maxFps: number
  // Browser-specific notes (e.g., "Safari only supports H.265 via MSE")
  notes?: string
}

// Detailed codec support map for intelligent codec selection
export interface CodecSupport {
  h264: CodecCapability
  h265: CodecCapability
  vp9: CodecCapability
  av1: CodecCapability
  // Ordered list of preferred codecs (best compression first)
  preferredOrder: ('h264' | 'h265' | 'vp9' | 'av1')[]
}

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
  codecSupport: CodecSupport
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

export interface ConnectionInfo {
  type: string
  effectiveType: string
  downlink: number
  rtt: number
  saveData: boolean
}

export interface BatteryInfo {
  level: number
  charging: boolean
  chargingTime: number
  dischargingTime: number
}

export interface ScreenInfo {
  width: number
  height: number
  pixelRatio: number
  availWidth: number
  availHeight: number
  orientation: string
}

export interface PerformanceInfo {
  cpuCores: number
  memoryGB: number
  isTouchDevice: boolean
}

export type DeviceType = 'mobile' | 'tablet' | 'desktop' | 'tv' | 'unknown'
