import { useState, useEffect, useRef, useCallback } from 'react'
import {
  createNetworkMonitorState,
  recordSample as recordNetworkSample,
  recordStall as recordNetworkStall,
  pruneOldSamples,
  startMonitoring,
  stopMonitoring,
  calculateStats,
  getQualityRecommendation,
  DEFAULT_NETWORK_CONFIG,
} from '../network/NetworkMonitor'
import type { NetworkStats, NetworkMonitorState, NetworkMonitorConfig } from '../network/NetworkMonitor'

export interface UseNetworkMonitorOptions {
  enabled?: boolean
  config?: Partial<NetworkMonitorConfig>
  onQualityRecommendation?: (quality: string, reason: string) => void
}

export interface UseNetworkMonitorReturn {
  stats: NetworkStats | null
  recommendedQuality: string | null
  recommendationReason: string | null
  recordSample: (bytes: number, durationMs: number, wasStall?: boolean) => void
  recordStall: () => void
  isMonitoring: boolean
  sampleCount: number
}

/**
 * React hook for network monitoring during video playback
 *
 * Usage:
 * ```tsx
 * const { stats, recordSample, recordStall } = useNetworkMonitor({ enabled: true })
 *
 * // In HLS.js event handlers:
 * hls.on(Hls.Events.FRAG_LOADED, (_, data) => {
 *   recordSample(data.frag.stats.total, data.frag.stats.loading.end - data.frag.stats.loading.start)
 * })
 * ```
 */
export const useNetworkMonitor = (
  options: UseNetworkMonitorOptions = {}
): UseNetworkMonitorReturn => {
  const { enabled = true, config, onQualityRecommendation } = options
  const [stats, setStats] = useState<NetworkStats | null>(null)
  const [recommendedQuality, setRecommendedQuality] = useState<string | null>(null)
  const [recommendationReason, setRecommendationReason] = useState<string | null>(null)
  const [sampleCount, setSampleCount] = useState(0)
  const [isMonitoring, setIsMonitoring] = useState(false)

  const stateRef = useRef<NetworkMonitorState>(createNetworkMonitorState())
  const configRef = useRef<NetworkMonitorConfig>({ ...DEFAULT_NETWORK_CONFIG, ...config })
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!enabled) {
      // Clean up if disabled
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      stateRef.current = stopMonitoring(stateRef.current)
      setStats(null)
      setRecommendedQuality(null)
      setRecommendationReason(null)
      setSampleCount(0)
      setIsMonitoring(false)
      return
    }

    // Start monitoring
    stateRef.current = startMonitoring(stateRef.current)
    setIsMonitoring(true)

    // Periodic stats calculation
    intervalRef.current = setInterval(() => {
      stateRef.current = pruneOldSamples(stateRef.current, configRef.current)
      const newStats = calculateStats(stateRef.current, configRef.current)

      if (newStats) {
        setStats(newStats)
        setSampleCount(stateRef.current.samples.length)

        const recommendation = getQualityRecommendation(newStats)
        setRecommendedQuality(recommendation.quality)
        setRecommendationReason(recommendation.reason)
        onQualityRecommendation?.(recommendation.quality, recommendation.reason)
      }
    }, 2000)

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      stateRef.current = stopMonitoring(stateRef.current)
    }
  }, [enabled, onQualityRecommendation])

  const recordSample = useCallback(
    (bytes: number, durationMs: number, wasStall?: boolean) => {
      stateRef.current = recordNetworkSample(stateRef.current, bytes, durationMs, wasStall)
      // Update sample count immediately so UI shows progress
      setSampleCount(stateRef.current.samples.length)
    },
    []
  )

  const recordStall = useCallback(() => {
    stateRef.current = recordNetworkStall(stateRef.current)
  }, [])

  return {
    stats,
    recommendedQuality,
    recommendationReason,
    recordSample,
    recordStall,
    isMonitoring,
    sampleCount,
  }
}

export default useNetworkMonitor
