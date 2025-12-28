/**
 * Device Profile Generator
 *
 * Creates a stable hash representing the client's playback capabilities.
 * Used to store device-specific playback preferences (e.g., quality, audio track).
 *
 * Different browsers on the same machine may have different profiles because:
 * - Firefox supports HEVC/Dolby Vision natively on some platforms
 * - Chrome requires hardware decode or doesn't support HEVC at all
 * - Safari has native HLS and different codec support
 */

import { detectCodecSupportSync, detectHDRDisplaySync } from './MediaCapabilityDetector'

export interface DeviceProfileData {
  // Core codec support (affects what can be direct played vs transcoded)
  h264: boolean
  h265: boolean
  vp9: boolean
  av1: boolean
  // HDR capability (affects tone mapping requirements)
  hdrDisplay: boolean
  // Browser family (Safari has native HLS, others use HLS.js)
  browserFamily: 'safari' | 'chrome' | 'firefox' | 'edge' | 'other'
}

/**
 * Detect browser family from user agent
 */
const detectBrowserFamily = (): DeviceProfileData['browserFamily'] => {
  const ua = navigator.userAgent.toLowerCase()

  // Order matters - check more specific browsers first
  if (ua.includes('edg/')) {
    return 'edge'
  }
  if (ua.includes('firefox')) {
    return 'firefox'
  }
  if (ua.includes('safari') && !ua.includes('chrome') && !ua.includes('chromium')) {
    return 'safari'
  }
  if (ua.includes('chrome') || ua.includes('chromium')) {
    return 'chrome'
  }
  return 'other'
}

/**
 * Generate device profile data from current browser capabilities
 */
export const getDeviceProfileData = (): DeviceProfileData => {
  const codecs = detectCodecSupportSync()
  const hdr = detectHDRDisplaySync()

  return {
    h264: codecs.h264.supported,
    h265: codecs.h265.supported,
    vp9: codecs.vp9.supported,
    av1: codecs.av1.supported,
    hdrDisplay: hdr.displaySupportsHDR,
    browserFamily: detectBrowserFamily(),
  }
}

/**
 * Generate a short stable hash from device profile data.
 * This is used as the device_profile key in the database.
 *
 * Format: {browser}-{codecs}-{hdr}
 * Examples:
 * - "chrome-h264-sdr" (Chrome without HEVC, no HDR)
 * - "firefox-h264h265-hdr" (Firefox with HEVC support, HDR display)
 * - "safari-h264h265av1-hdr" (Safari with full codec support, HDR)
 */
export const generateDeviceProfileHash = (data?: DeviceProfileData): string => {
  const profile = data ?? getDeviceProfileData()

  // Build codec string (sorted for consistency)
  const codecs: string[] = []
  if (profile.h264) {
    codecs.push('h264')
  }
  if (profile.h265) {
    codecs.push('h265')
  }
  if (profile.vp9) {
    codecs.push('vp9')
  }
  if (profile.av1) {
    codecs.push('av1')
  }

  const codecStr = codecs.length > 0 ? codecs.join('') : 'none'
  const hdrStr = profile.hdrDisplay ? 'hdr' : 'sdr'

  return `${profile.browserFamily}-${codecStr}-${hdrStr}`
}

/**
 * Get the current device's profile hash.
 * Cached after first call for performance.
 */
let cachedProfileHash: string | null = null

export const getDeviceProfileHash = (): string => {
  if (cachedProfileHash === null) {
    cachedProfileHash = generateDeviceProfileHash()
  }
  return cachedProfileHash
}

/**
 * Clear the cached profile hash (useful for testing or after capability changes)
 */
export const clearDeviceProfileCache = (): void => {
  cachedProfileHash = null
}
