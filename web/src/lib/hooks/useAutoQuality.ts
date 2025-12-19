/**
 * useAutoQuality Hook
 *
 * Provides network monitoring for the debug stats panel.
 * Records download samples and stall events to calculate bandwidth statistics.
 *
 * Note: The name is historical - this hook previously managed HLS.js ABR mode.
 * Now we use single-quality streaming with backend-selected quality, so the
 * "auto mode" functionality is no longer used. Only network monitoring remains.
 */

import type Hls from 'hls.js'
import { useNetworkMonitor } from './useNetworkMonitor'

export interface UseAutoQualityOptions {
  enabled?: boolean
  hlsInstance: Hls | null
}

export interface UseAutoQualityReturn {
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
 * Network monitoring hook for video playback.
 *
 * Single-Quality Architecture:
 * - Backend picks optimal quality based on client capabilities
 * - User can override via quality picker (triggers stream reload)
 * - Each quality change restarts FFmpeg from current position
 *
 * This hook provides network stats for the debug overlay only.
 */
export const useAutoQuality = (options: UseAutoQualityOptions): UseAutoQualityReturn => {
  const { enabled = true } = options

  // Network monitoring - for stats display in debug overlay
  const {
    stats: networkStats,
    recordSample,
    recordStall,
    sampleCount,
  } = useNetworkMonitor({
    enabled,
  })

  return {
    recordSample,
    recordStall,
    networkStats,
    sampleCount,
  }
}

export default useAutoQuality
