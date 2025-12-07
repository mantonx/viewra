/**
 * Shared utility functions for quick device/connection checks
 * Used by analytics and network monitoring during playback
 */

import type { ExtendedNavigator } from './browser-types'
import type { CodecSupport } from './types'

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

/**
 * Converts CodecSupport to a comma-separated header value for API requests.
 * Used to inform the backend which codecs the client can decode.
 */
export const getSupportedCodecsHeader = (codecSupport: CodecSupport | null): string => {
  if (!codecSupport) {
    return 'h264' // Safe fallback
  }
  const supported: string[] = []
  if (codecSupport.h264.supported) {
    supported.push('h264')
  }
  if (codecSupport.h265.supported) {
    supported.push('h265')
  }
  if (codecSupport.vp9.supported) {
    supported.push('vp9')
  }
  if (codecSupport.av1.supported) {
    supported.push('av1')
  }
  return supported.join(',')
}
