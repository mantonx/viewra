import type { BatteryInfo, DeviceType, PerformanceInfo, ScreenInfo } from './types'
import type { ExtendedNavigator } from './browser-types'

const MOBILE_BREAKPOINT = 768
const TABLET_MIN_BREAKPOINT = 768
const TABLET_MAX_BREAKPOINT = 1024
const LOW_BATTERY_THRESHOLD = 0.2

const isTVDevice = (ua: string): boolean => {
  const tvKeywords = ['tv', 'smarttv', 'googletv', 'appletv']
  return tvKeywords.some(keyword => ua.includes(keyword))
}

const isMobileOrTablet = (ua: string): boolean => {
  return /android|iphone|ipod|mobile/.test(ua)
}

const isIPad = (ua: string): boolean => {
  return /ipad|macintosh/.test(ua) && navigator.maxTouchPoints > 1
}

const isTabletByScreenSize = (minDimension: number): boolean => {
  return (
    minDimension >= TABLET_MIN_BREAKPOINT &&
    minDimension < TABLET_MAX_BREAKPOINT &&
    window.matchMedia('(pointer: coarse)').matches
  )
}

export const detectDeviceType = (): DeviceType => {
  const ua = navigator.userAgent.toLowerCase()
  const { width, height } = window.screen
  const minDimension = Math.min(width, height)

  if (isTVDevice(ua)) {
    return 'tv'
  }

  if (isMobileOrTablet(ua)) {
    return minDimension < MOBILE_BREAKPOINT ? 'mobile' : 'tablet'
  }

  if (isIPad(ua)) {
    return 'tablet'
  }

  if (isTabletByScreenSize(minDimension)) {
    return 'tablet'
  }

  return 'desktop'
}

export const getScreenInfo = (): ScreenInfo => ({
  width: window.screen.width,
  height: window.screen.height,
  pixelRatio: window.devicePixelRatio || 1,
  availWidth: window.screen.availWidth,
  availHeight: window.screen.availHeight,
  orientation: window.screen.orientation?.type || 'unknown',
})

export const getPerformanceInfo = (): PerformanceInfo => {
  const extendedNav = navigator as ExtendedNavigator
  return {
    cpuCores: navigator.hardwareConcurrency || 2,
    memoryGB: extendedNav.deviceMemory ?? -1,
    isTouchDevice: 'ontouchstart' in window || navigator.maxTouchPoints > 0,
  }
}

const getDefaultBatteryInfo = (): BatteryInfo => ({
  level: -1,
  charging: false,
  chargingTime: -1,
  dischargingTime: -1,
})

export const getBatteryInfo = async (): Promise<BatteryInfo> => {
  const extendedNav = navigator as ExtendedNavigator

  if (!extendedNav.getBattery) {
    return getDefaultBatteryInfo()
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
    return getDefaultBatteryInfo()
  }
}

export const isLowPowerMode = (batteryInfo: BatteryInfo): boolean => {
  // iOS 9+ Low Power Mode detection (indirect)
  // When enabled, requestAnimationFrame throttles to ~30fps
  // We use battery level as a heuristic
  return (
    batteryInfo.level > 0 &&
    batteryInfo.level < LOW_BATTERY_THRESHOLD &&
    !batteryInfo.charging
  )
}

