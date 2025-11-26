/**
 * Shared utility functions for quick device/connection checks
 * Used by analytics and network monitoring during playback
 */

import type { ExtendedNavigator } from './browser-types'

// Re-export detectDeviceType from DeviceDetector for convenience
export { detectDeviceType } from './DeviceDetector'

/**
 * Get the Navigator Connection API object (browser-specific)
 */
const getNavigatorConnection = () => {
  const extendedNav = navigator as ExtendedNavigator
  return extendedNav.connection || extendedNav.mozConnection || extendedNav.webkitConnection
}

/**
 * Get connection type from Navigator Connection API
 * Returns effectiveType (4g, 3g, etc.) for quick checks during playback
 */
export const getConnectionType = (): string => {
  return getNavigatorConnection()?.effectiveType ?? 'unknown'
}

/**
 * Check if connection is metered (data saving mode)
 */
export const isMeteredConnection = (): boolean => {
  return getNavigatorConnection()?.saveData ?? false
}
