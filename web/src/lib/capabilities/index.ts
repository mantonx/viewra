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
  detectCodecSupportSync,
  detectHardwareAcceleration,
  detectMaxDecodingProfile,
  probeAllCodecSupport,
  detectHDRCapability,
  detectHDRDisplaySync,
  setHDROverride,
} from './MediaCapabilityDetector'

// Shared utilities
export { getConnectionType, isMeteredConnection, getSupportedCodecsHeader, getHDRCapabilityHeader } from './utils'

// Device profile (for device-specific preferences)
export {
  getDeviceProfileData,
  getDeviceProfileHash,
  generateDeviceProfileHash,
  clearDeviceProfileCache,
} from './DeviceProfile'
export type { DeviceProfileData } from './DeviceProfile'

// Types
export type {
  ClientCapabilities,
  CodecCapability,
  CodecSupport,
  HDRCapability,
  NetworkMeasurement,
  ConnectionInfo,
  BatteryInfo,
  ScreenInfo,
  PerformanceInfo,
  DeviceType,
} from './types'
