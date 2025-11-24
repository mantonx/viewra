import type { NetworkMeasurement, ConnectionInfo } from './types'
import type { ExtendedNavigator } from './browser-types'

export class NetworkDetector {
  private static readonly MAX_MEASUREMENTS = 10
  private static readonly MILLISECONDS_PER_SECOND = 1000
  private static readonly BITS_PER_BYTE = 8
  private static readonly BITS_PER_MEGABIT = 1_000_000

  private measurements: NetworkMeasurement[] = []

  async detectConnectionType(): Promise<ConnectionInfo> {
    const extendedNav = navigator as ExtendedNavigator
    const connection =
      extendedNav.connection || extendedNav.mozConnection || extendedNav.webkitConnection

    if (!connection) {
      return this.getDefaultConnectionInfo()
    }

    return {
      type: connection.type ?? 'unknown',
      effectiveType: connection.effectiveType ?? 'unknown',
      downlink: connection.downlink ?? -1,
      rtt: connection.rtt ?? -1,
      saveData: connection.saveData ?? false,
    }
  }

  private getDefaultConnectionInfo(): ConnectionInfo {
    return {
      type: 'unknown',
      effectiveType: 'unknown',
      downlink: -1,
      rtt: -1,
      saveData: false,
    }
  }

  async measureNetworkSpeed(): Promise<NetworkMeasurement> {
    const startTime = performance.now()

    try {
      const response = await this.fetchSpeedTestChunk()
      const blob = await response.blob()

      const duration = (performance.now() - startTime) / NetworkDetector.MILLISECONDS_PER_SECOND
      const speedMbps = this.calculateSpeedMbps(blob.size, duration)
      const latencyMs = this.extractLatency(response, startTime)

      const measurement = this.createMeasurement(speedMbps, latencyMs)
      this.addMeasurement(measurement)

      return measurement
    } catch (error) {
      console.error('Network speed measurement failed:', error)
      return this.getFailedMeasurement()
    }
  }

  private async fetchSpeedTestChunk(): Promise<Response> {
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

  private calculateSpeedMbps(bytesDownloaded: number, durationSeconds: number): number {
    const bitsDownloaded = bytesDownloaded * NetworkDetector.BITS_PER_BYTE
    const speedMbps = bitsDownloaded / durationSeconds / NetworkDetector.BITS_PER_MEGABIT
    return Math.round(speedMbps * 100) / 100
  }

  private extractLatency(response: Response, startTime: number): number {
    const serverTiming = response.headers.get('Server-Timing')
    const parsedLatency = this.parseLatency(serverTiming)

    if (parsedLatency !== null) {
      return parsedLatency
    }

    // Fallback: estimate as half of total time
    return Math.round((performance.now() - startTime) / 2)
  }

  private createMeasurement(speedMbps: number, latencyMs: number): NetworkMeasurement {
    return {
      speedMbps,
      latencyMs,
      jitter: this.measurements.length > 1 ? this.calculateJitter() : 0,
      timestamp: new Date(),
    }
  }

  private addMeasurement(measurement: NetworkMeasurement): void {
    this.measurements.push(measurement)

    if (this.measurements.length > NetworkDetector.MAX_MEASUREMENTS) {
      this.measurements.shift()
    }
  }

  private getFailedMeasurement(): NetworkMeasurement {
    return {
      speedMbps: -1,
      latencyMs: -1,
      jitter: -1,
      timestamp: new Date(),
    }
  }

  private calculateJitter(): number {
    if (this.measurements.length < 2) {
      return 0
    }

    const speeds = this.measurements.map(m => m.speedMbps)
    const jitterSum = speeds
      .slice(1)
      .reduce((sum, speed, index) => sum + Math.abs(speed - speeds[index]), 0)

    return jitterSum / (speeds.length - 1)
  }

  getAverageSpeed(): number {
    if (this.measurements.length === 0) {
      return -1
    }

    const sum = this.measurements.reduce((acc, m) => acc + m.speedMbps, 0)
    return sum / this.measurements.length
  }

  private parseLatency(serverTiming: string | null): number | null {
    if (!serverTiming) {
      return null
    }

    const match = serverTiming.match(/dur=(\d+\.?\d*)/)
    return match ? parseFloat(match[1]) : null
  }
}
