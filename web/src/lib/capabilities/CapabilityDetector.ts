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
      lowPowerMode: this.deviceDetector.isLowPowerMode(batteryInfo),
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
