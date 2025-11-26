/**
 * NetworkMonitor - Monitors network conditions during video playback
 *
 * Collects throughput samples from HLS.js fragment loads and calculates
 * statistics used for intelligent quality switching decisions.
 */

import { getConnectionType, isMeteredConnection } from '@/lib/capabilities'

export interface NetworkSample {
  timestamp: number
  bytesDownloaded: number
  durationMs: number
  throughputMbps: number
  latencyMs: number
  wasStall: boolean
}

export interface NetworkStats {
  currentThroughputMbps: number
  averageThroughputMbps: number
  minThroughputMbps: number
  maxThroughputMbps: number
  stability: number // 0-1 (1 = very stable)
  stallCount: number
  lastStallTime: number | null
  trend: 'improving' | 'stable' | 'degrading'
  connectionType: string
  isMetered: boolean
}

export interface NetworkMonitorConfig {
  sampleWindowMs: number
  minSamplesForStats: number
  stabilityThreshold: number
}

export interface NetworkMonitorState {
  samples: NetworkSample[]
  isMonitoring: boolean
}

export const DEFAULT_NETWORK_CONFIG: NetworkMonitorConfig = {
  sampleWindowMs: 30000,
  minSamplesForStats: 3,
  stabilityThreshold: 0.3,
}

/**
 * Create initial monitor state
 */
export const createNetworkMonitorState = (): NetworkMonitorState => ({
  samples: [],
  isMonitoring: false,
})

/**
 * Record a network sample from HLS.js fragment load
 */
export const recordSample = (
  state: NetworkMonitorState,
  bytesDownloaded: number,
  durationMs: number,
  wasStall = false
): NetworkMonitorState => {
  if (!state.isMonitoring || durationMs <= 0) {
    return state
  }

  const throughputMbps = (bytesDownloaded * 8) / (durationMs * 1000)

  return {
    ...state,
    samples: [
      ...state.samples,
      {
        timestamp: Date.now(),
        bytesDownloaded,
        durationMs,
        throughputMbps,
        latencyMs: 0,
        wasStall,
      },
    ],
  }
}

/**
 * Record a buffer stall event
 */
export const recordStall = (state: NetworkMonitorState): NetworkMonitorState => {
  if (state.samples.length > 0) {
    const samples = [...state.samples]
    samples[samples.length - 1] = { ...samples[samples.length - 1], wasStall: true }
    return { ...state, samples }
  }

  return {
    ...state,
    samples: [
      ...state.samples,
      {
        timestamp: Date.now(),
        bytesDownloaded: 0,
        durationMs: 0,
        throughputMbps: 0,
        latencyMs: 0,
        wasStall: true,
      },
    ],
  }
}

/**
 * Remove samples outside the window
 */
export const pruneOldSamples = (
  state: NetworkMonitorState,
  config: NetworkMonitorConfig = DEFAULT_NETWORK_CONFIG
): NetworkMonitorState => {
  const cutoff = Date.now() - config.sampleWindowMs
  return {
    ...state,
    samples: state.samples.filter((s) => s.timestamp > cutoff),
  }
}

/**
 * Start monitoring
 */
export const startMonitoring = (_state: NetworkMonitorState): NetworkMonitorState => ({
  samples: [],
  isMonitoring: true,
})

/**
 * Stop monitoring
 */
export const stopMonitoring = (state: NetworkMonitorState): NetworkMonitorState => ({
  ...state,
  isMonitoring: false,
})

const calculateTrend = (throughputs: number[]): 'improving' | 'stable' | 'degrading' => {
  if (throughputs.length < 4) {
    return 'stable'
  }

  const midpoint = Math.floor(throughputs.length / 2)
  const firstHalf = throughputs.slice(0, midpoint)
  const secondHalf = throughputs.slice(midpoint)

  const firstHalfAvg = firstHalf.reduce((a, b) => a + b, 0) / firstHalf.length
  const secondHalfAvg = secondHalf.reduce((a, b) => a + b, 0) / secondHalf.length

  if (secondHalfAvg > firstHalfAvg * 1.15) {
    return 'improving'
  }
  if (secondHalfAvg < firstHalfAvg * 0.85) {
    return 'degrading'
  }
  return 'stable'
}


/**
 * Calculate network statistics from samples
 */
export const calculateStats = (
  state: NetworkMonitorState,
  config: NetworkMonitorConfig = DEFAULT_NETWORK_CONFIG
): NetworkStats | null => {
  if (state.samples.length < config.minSamplesForStats) {
    return null
  }

  const validSamples = state.samples.filter((s) => s.throughputMbps > 0)
  const connectionType = getConnectionType()
  const isMetered = isMeteredConnection()

  if (validSamples.length === 0) {
    return {
      currentThroughputMbps: 0,
      averageThroughputMbps: 0,
      minThroughputMbps: 0,
      maxThroughputMbps: 0,
      stability: 0,
      stallCount: state.samples.filter((s) => s.wasStall).length,
      lastStallTime: null,
      trend: 'stable',
      connectionType,
      isMetered,
    }
  }

  const throughputs = validSamples.map((s) => s.throughputMbps)
  const sum = throughputs.reduce((a, b) => a + b, 0)
  const avg = sum / throughputs.length
  const min = Math.min(...throughputs)
  const max = Math.max(...throughputs)

  // Stability: inverse of coefficient of variation
  const variance = throughputs.reduce((acc, t) => acc + Math.pow(t - avg, 2), 0) / throughputs.length
  const stdDev = Math.sqrt(variance)
  const cv = avg > 0 ? stdDev / avg : 1
  const stability = Math.max(0, Math.min(1, 1 - cv))

  const stallCount = state.samples.filter((s) => s.wasStall).length
  const lastStall = state.samples.filter((s) => s.wasStall).pop()

  return {
    currentThroughputMbps: throughputs[throughputs.length - 1],
    averageThroughputMbps: avg,
    minThroughputMbps: min,
    maxThroughputMbps: max,
    stability,
    stallCount,
    lastStallTime: lastStall?.timestamp ?? null,
    trend: calculateTrend(throughputs),
    connectionType,
    isMetered,
  }
}

/**
 * Get quality recommendation based on network stats
 */
export const getQualityRecommendation = (
  stats: NetworkStats
): { quality: string; reason: string } => {
  const safeBitrateMbps = stats.averageThroughputMbps * 0.7

  if (stats.stallCount > 2 || stats.trend === 'degrading') {
    const quality = safeBitrateMbps < 2 ? '360p' : safeBitrateMbps < 4 ? '480p' : '720p'
    const reason =
      stats.stallCount > 2
        ? `Buffering detected (${stats.stallCount} stalls)`
        : 'Network speed decreasing'
    return { quality, reason }
  }

  if (stats.stability < 0.5) {
    const quality = safeBitrateMbps < 3 ? '480p' : '720p'
    return { quality, reason: 'Unstable network - using conservative quality' }
  }

  let quality: string
  if (safeBitrateMbps >= 20) {
    quality = '4k'
  } else if (safeBitrateMbps >= 8) {
    quality = '1080p'
  } else if (safeBitrateMbps >= 4) {
    quality = '720p'
  } else if (safeBitrateMbps >= 1.5) {
    quality = '480p'
  } else {
    quality = '360p'
  }

  return { quality, reason: `Network supports ${stats.averageThroughputMbps.toFixed(1)} Mbps` }
}
