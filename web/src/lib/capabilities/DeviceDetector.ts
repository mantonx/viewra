import type { BatteryInfo, DeviceType, PerformanceInfo, ScreenInfo } from './types'
import type { ExtendedNavigator } from './browser-types'

export class DeviceDetector {
  private static readonly MOBILE_BREAKPOINT = 768
  private static readonly TABLET_MIN_BREAKPOINT = 768
  private static readonly TABLET_MAX_BREAKPOINT = 1024
  private static readonly LOW_BATTERY_THRESHOLD = 0.2

  detectDeviceType(): DeviceType {
    const ua = navigator.userAgent.toLowerCase()
    const { width, height } = window.screen
    const minDimension = Math.min(width, height)

    if (this.isTVDevice(ua)) {
      return 'tv'
    }

    if (this.isMobileOrTablet(ua)) {
      return minDimension < DeviceDetector.MOBILE_BREAKPOINT ? 'mobile' : 'tablet'
    }

    if (this.isIPad(ua)) {
      return 'tablet'
    }

    if (this.isTabletByScreenSize(minDimension)) {
      return 'tablet'
    }

    return 'desktop'
  }

  private isTVDevice(ua: string): boolean {
    const tvKeywords = ['tv', 'smarttv', 'googletv', 'appletv']
    return tvKeywords.some(keyword => ua.includes(keyword))
  }

  private isMobileOrTablet(ua: string): boolean {
    return /android|iphone|ipod|mobile/.test(ua)
  }

  private isIPad(ua: string): boolean {
    return /ipad|macintosh/.test(ua) && navigator.maxTouchPoints > 1
  }

  private isTabletByScreenSize(minDimension: number): boolean {
    return (
      minDimension >= DeviceDetector.TABLET_MIN_BREAKPOINT &&
      minDimension < DeviceDetector.TABLET_MAX_BREAKPOINT &&
      window.matchMedia('(pointer: coarse)').matches
    )
  }

  getScreenInfo(): ScreenInfo {
    return {
      width: window.screen.width,
      height: window.screen.height,
      pixelRatio: window.devicePixelRatio || 1,
      availWidth: window.screen.availWidth,
      availHeight: window.screen.availHeight,
      orientation: window.screen.orientation?.type || 'unknown',
    }
  }

  getPerformanceInfo(): PerformanceInfo {
    const extendedNav = navigator as ExtendedNavigator
    return {
      cpuCores: navigator.hardwareConcurrency || 2,
      memoryGB: extendedNav.deviceMemory ?? -1,
      isTouchDevice: 'ontouchstart' in window || navigator.maxTouchPoints > 0,
    }
  }

  async getBatteryInfo(): Promise<BatteryInfo> {
    const extendedNav = navigator as ExtendedNavigator

    if (!extendedNav.getBattery) {
      return this.getDefaultBatteryInfo()
    }

    try {
      const battery = await extendedNav.getBattery()
      return {
        level: battery.level,
        charging: battery.charging,
        chargingTime: battery.chargingTime,
        dischargingTime: battery.dischargingTime,
      }
    } catch {
      return this.getDefaultBatteryInfo()
    }
  }

  private getDefaultBatteryInfo(): BatteryInfo {
    return { level: -1, charging: false, chargingTime: -1, dischargingTime: -1 }
  }

  isLowPowerMode(batteryInfo: BatteryInfo): boolean {
    // iOS 9+ Low Power Mode detection (indirect)
    // When enabled, requestAnimationFrame throttles to ~30fps
    // We use battery level as a heuristic
    return (
      batteryInfo.level > 0 &&
      batteryInfo.level < DeviceDetector.LOW_BATTERY_THRESHOLD &&
      !batteryInfo.charging
    )
  }
}
