// Main capability detection
export {
  capabilityDetector,
  detectCapabilities,
  startNetworkMonitoring,
  createCapabilityDetectorState,
} from './CapabilityDetector'
export type { CapabilityDetectorState } from './CapabilityDetector'

// Device detection
export {
  detectDeviceType,
  getScreenInfo,
  getPerformanceInfo,
  getBatteryInfo,
  isLowPowerMode,
} from './DeviceDetector'

// Network detection
export {
  detectConnectionType,
  measureNetworkSpeed,
  getAverageSpeed,
  createNetworkDetectorState,
} from './NetworkDetector'
export type { NetworkDetectorState } from './NetworkDetector'

// Media capability detection
export {
  detectCodecSupport,
  detectHardwareAcceleration,
  detectMaxDecodingProfile,
} from './MediaCapabilityDetector'

// Shared utilities
export { getConnectionType, isMeteredConnection } from './utils'

// Types
export type {
  ClientCapabilities,
  CodecCapability,
  CodecSupport,
  NetworkMeasurement,
  ConnectionInfo,
  BatteryInfo,
  ScreenInfo,
  PerformanceInfo,
  DeviceType,
} from './types'
