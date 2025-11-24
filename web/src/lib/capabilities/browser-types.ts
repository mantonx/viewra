// Type definitions for browser APIs that don't have proper TypeScript definitions

export interface NetworkInformation {
  type?: string
  effectiveType?: string
  downlink?: number
  rtt?: number
  saveData?: boolean
}

export interface ExtendedNavigator extends Navigator {
  connection?: NetworkInformation
  mozConnection?: NetworkInformation
  webkitConnection?: NetworkInformation
  deviceMemory?: number
  getBattery?: () => Promise<BatteryManager>
}

export interface BatteryManager {
  level: number
  charging: boolean
  chargingTime: number
  dischargingTime: number
}
