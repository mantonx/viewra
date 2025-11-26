import {
  detectDeviceType,
  getScreenInfo,
  getPerformanceInfo,
  getBatteryInfo,
  isLowPowerMode,
} from './DeviceDetector'
import {
  detectConnectionType,
  measureNetworkSpeed,
  createNetworkDetectorState,
} from './NetworkDetector'
import type { NetworkDetectorState } from './NetworkDetector'
import {
  detectCodecSupport,
  detectHardwareAcceleration,
  detectMaxDecodingProfile,
} from './MediaCapabilityDetector'
import type { ClientCapabilities } from './types'

const CACHE_EXPIRY_MS = 5 * 60 * 1000 // 5 minutes

export interface CapabilityDetectorState {
  cachedCapabilities: ClientCapabilities | null
  networkState: NetworkDetectorState
}

export const createCapabilityDetectorState = (): CapabilityDetectorState => ({
  cachedCapabilities: null,
  networkState: createNetworkDetectorState(),
})

const mapConnectionType = (type: string): ClientCapabilities['connectionType'] => {
  const typeMap: Record<string, ClientCapabilities['connectionType']> = {
    'wifi': 'wifi',
    'ethernet': 'ethernet',
    'cellular': '4g',
    '4g': '4g',
    '5g': '5g',
    '3g': '3g',
    '2g': '2g',
    'slow-2g': 'slow-2g',
  }
  return typeMap[type] || 'unknown'
}

const detectBrowser = (): string => {
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes('firefox')) {
    return 'firefox'
  }
  if (ua.includes('edg/')) {
    return 'edge'
  }
  if (ua.includes('chrome')) {
    return 'chrome'
  }
  if (ua.includes('safari')) {
    return 'safari'
  }
  return 'unknown'
}

const detectBrowserVersion = (): string => {
  const ua = navigator.userAgent
  const match = ua.match(/(firefox|chrome|safari|edg)\/(\d+)/i)
  return match ? match[2] : 'unknown'
}

export const detectCapabilities = async (
  state: CapabilityDetectorState,
  forceRefresh: boolean = false
): Promise<{ state: CapabilityDetectorState; capabilities: ClientCapabilities }> => {
  // Return cached if still valid and not forcing refresh
  if (
    !forceRefresh &&
    state.cachedCapabilities &&
    Date.now() - state.cachedCapabilities.detectedAt.getTime() < CACHE_EXPIRY_MS
  ) {
    return { state, capabilities: state.cachedCapabilities }
  }

  // Parallel detection for speed
  const [
    deviceType,
    screenInfo,
    performanceInfo,
    batteryInfo,
    connectionInfo,
    networkResult,
    codecSupport,
    hardwareAccel,
    maxProfile,
  ] = await Promise.all([
    Promise.resolve(detectDeviceType()),
    Promise.resolve(getScreenInfo()),
    Promise.resolve(getPerformanceInfo()),
    getBatteryInfo(),
    detectConnectionType(),
    measureNetworkSpeed(state.networkState),
    detectCodecSupport(),
    detectHardwareAcceleration(),
    detectMaxDecodingProfile(),
  ])

  const capabilities: ClientCapabilities = {
    // Network
    networkSpeedMbps: networkResult.measurement.speedMbps,
    connectionType: mapConnectionType(connectionInfo.type),
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
    lowPowerMode: isLowPowerMode(batteryInfo),
    isCharging: batteryInfo.charging,

    // Media
    codecSupport,
    hardwareAcceleration: hardwareAccel,
    maxDecodingProfile: maxProfile,

    // Browser
    userAgent: navigator.userAgent,
    browserName: detectBrowser(),
    browserVersion: detectBrowserVersion(),

    // Metadata
    detectedAt: new Date(),
  }

  return {
    state: {
      cachedCapabilities: capabilities,
      networkState: networkResult.state,
    },
    capabilities,
  }
}

export const startNetworkMonitoring = async (
  state: CapabilityDetectorState,
  intervalMs: number = 60000,
  callback: (speed: number) => void
): Promise<{ state: CapabilityDetectorState; intervalId: ReturnType<typeof setInterval> }> => {
  // Initial measurement
  const initialResult = await measureNetworkSpeed(state.networkState)
  callback(initialResult.measurement.speedMbps)

  let currentNetworkState = initialResult.state

  // Periodic measurements
  const intervalId = setInterval(async () => {
    const result = await measureNetworkSpeed(currentNetworkState)
    currentNetworkState = result.state
    callback(result.measurement.speedMbps)
  }, intervalMs)

  return {
    state: { ...state, networkState: initialResult.state },
    intervalId,
  }
}

// Singleton state for convenience hook
let singletonState = createCapabilityDetectorState()

export const capabilityDetector = {
  async detectCapabilities(forceRefresh: boolean = false): Promise<ClientCapabilities> {
    const result = await detectCapabilities(singletonState, forceRefresh)
    singletonState = result.state
    return result.capabilities
  },

  async startNetworkMonitoring(
    intervalMs: number = 60000,
    callback: (speed: number) => void
  ): Promise<ReturnType<typeof setInterval>> {
    const result = await startNetworkMonitoring(singletonState, intervalMs, callback)
    singletonState = result.state
    return result.intervalId
  },
}
