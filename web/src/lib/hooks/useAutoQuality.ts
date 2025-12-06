/**
 * useAutoQuality Hook
 *
 * Simplified hook that only handles:
 * 1. Network monitoring (for stats display)
 * 2. Auto mode state (whether HLS.js ABR is enabled)
 *
 * Quality switching is delegated entirely to HLS.js ABR when in auto mode.
 * This removes the competing quality control systems that caused race conditions.
 */

import { loadVideoPreferences } from '@/lib/preferences'
import type Hls from 'hls.js'
import { useCallback, useState } from 'react'
import { useNetworkMonitor } from './useNetworkMonitor'

export interface UseAutoQualityOptions {
  enabled?: boolean
  hlsInstance: Hls | null
}

export interface UseAutoQualityReturn {
  /** Whether auto quality (HLS.js ABR) is enabled */
  isAutoMode: boolean
  /** Toggle auto mode on/off */
  setAutoMode: (auto: boolean) => void
  /** Record a network sample (bytes downloaded, time taken) */
  recordSample: (bytes: number, durationMs: number, wasStall?: boolean) => void
  /** Record a playback stall */
  recordStall: () => void
  /** Current network statistics */
  networkStats: import('../network/NetworkMonitor').NetworkStats | null
  /** Number of samples collected */
  sampleCount: number
}

/**
 * Simplified auto quality hook
 *
 * Previous implementation had a 2-second evaluation loop that competed with:
 * - Backend quality recommendation
 * - HLS.js internal ABR
 * - User manual selection
 *
 * New implementation:
 * - Auto mode = HLS.js ABR (currentLevel = -1)
 * - Manual mode = User's chosen level (currentLevel = specific index)
 * - Network monitoring for stats display only
 */
export const useAutoQuality = (options: UseAutoQualityOptions): UseAutoQualityReturn => {
  const { enabled = true, hlsInstance } = options

  // Load initial auto mode preference
  const [isAutoMode, setIsAutoMode] = useState(() => {
    const prefs = loadVideoPreferences()
    return prefs.autoQualityEnabled
  })

  // Network monitoring - for stats display in debug overlay
  const {
    stats: networkStats,
    recordSample,
    recordStall,
    sampleCount,
  } = useNetworkMonitor({
    enabled,
  })

  // NOTE: We intentionally do NOT set hlsInstance.currentLevel = -1 on mount.
  // HLS.js starts in auto mode by default with our abrEwmaDefaultEstimate.
  // Setting currentLevel = -1 would reset the ABR estimate and cause it to
  // start at low quality instead of the recommended quality.

  const setAutoMode = useCallback(
    (auto: boolean) => {
      setIsAutoMode(auto)

      if (hlsInstance) {
        if (auto) {
          // Enable HLS.js ABR
          hlsInstance.currentLevel = -1
        }
        // If turning off auto, don't change level - user will select manually
      }
    },
    [hlsInstance]
  )

  return {
    isAutoMode,
    setAutoMode,
    recordSample,
    recordStall,
    networkStats,
    sampleCount,
  }
}

export default useAutoQuality
