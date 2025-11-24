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
