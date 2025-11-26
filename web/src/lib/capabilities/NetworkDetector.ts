import type { NetworkMeasurement, ConnectionInfo } from './types'
import type { ExtendedNavigator } from './browser-types'

const MAX_MEASUREMENTS = 10
const MILLISECONDS_PER_SECOND = 1000
const BITS_PER_BYTE = 8
const BITS_PER_MEGABIT = 1_000_000

export interface NetworkDetectorState {
  measurements: NetworkMeasurement[]
}

export const createNetworkDetectorState = (): NetworkDetectorState => ({
  measurements: [],
})

const getDefaultConnectionInfo = (): ConnectionInfo => ({
  type: 'unknown',
  effectiveType: 'unknown',
  downlink: -1,
  rtt: -1,
  saveData: false,
})

export const detectConnectionType = async (): Promise<ConnectionInfo> => {
  const extendedNav = navigator as ExtendedNavigator
  const connection =
    extendedNav.connection || extendedNav.mozConnection || extendedNav.webkitConnection

  if (!connection) {
    return getDefaultConnectionInfo()
  }

  return {
    type: connection.type ?? 'unknown',
    effectiveType: connection.effectiveType ?? 'unknown',
    downlink: connection.downlink ?? -1,
    rtt: connection.rtt ?? -1,
    saveData: connection.saveData ?? false,
  }
}

const fetchSpeedTestChunk = async (): Promise<Response> => {
  const response = await fetch('/api/speedtest/chunk', {
    method: 'GET',
    headers: {
      'Cache-Control': 'no-cache',
      'Pragma': 'no-cache',
    },
  })

  if (!response.ok) {
    throw new Error(`Speed test failed: ${response.statusText}`)
  }

  return response
}

const calculateSpeedMbps = (bytesDownloaded: number, durationSeconds: number): number => {
  const bitsDownloaded = bytesDownloaded * BITS_PER_BYTE
  const speedMbps = bitsDownloaded / durationSeconds / BITS_PER_MEGABIT
  return Math.round(speedMbps * 100) / 100
}

const parseLatency = (serverTiming: string | null): number | null => {
  if (!serverTiming) {
    return null
  }

  const match = serverTiming.match(/dur=(\d+\.?\d*)/)
  return match ? parseFloat(match[1]) : null
}

const extractLatency = (response: Response, startTime: number): number => {
  const serverTiming = response.headers.get('Server-Timing')
  const parsedLatency = parseLatency(serverTiming)

  if (parsedLatency !== null) {
    return parsedLatency
  }

  // Fallback: estimate as half of total time
  return Math.round((performance.now() - startTime) / 2)
}

const calculateJitter = (measurements: NetworkMeasurement[]): number => {
  if (measurements.length < 2) {
    return 0
  }

  const speeds = measurements.map(m => m.speedMbps)
  const jitterSum = speeds
    .slice(1)
    .reduce((sum, speed, index) => sum + Math.abs(speed - speeds[index]), 0)

  return jitterSum / (speeds.length - 1)
}

const getFailedMeasurement = (): NetworkMeasurement => ({
  speedMbps: -1,
  latencyMs: -1,
  jitter: -1,
  timestamp: new Date(),
})

export const measureNetworkSpeed = async (
  state: NetworkDetectorState
): Promise<{ state: NetworkDetectorState; measurement: NetworkMeasurement }> => {
  const startTime = performance.now()

  try {
    const response = await fetchSpeedTestChunk()
    const blob = await response.blob()

    const duration = (performance.now() - startTime) / MILLISECONDS_PER_SECOND
    const speedMbps = calculateSpeedMbps(blob.size, duration)
    const latencyMs = extractLatency(response, startTime)

    const measurement: NetworkMeasurement = {
      speedMbps,
      latencyMs,
      jitter: state.measurements.length > 1 ? calculateJitter(state.measurements) : 0,
      timestamp: new Date(),
    }

    // Add measurement and trim to max
    const measurements = [...state.measurements, measurement]
    if (measurements.length > MAX_MEASUREMENTS) {
      measurements.shift()
    }

    return {
      state: { measurements },
      measurement,
    }
  } catch (error) {
    console.error('Network speed measurement failed:', error)
    return {
      state,
      measurement: getFailedMeasurement(),
    }
  }
}

export const getAverageSpeed = (state: NetworkDetectorState): number => {
  if (state.measurements.length === 0) {
    return -1
  }

  const sum = state.measurements.reduce((acc, m) => acc + m.speedMbps, 0)
  return sum / state.measurements.length
}

