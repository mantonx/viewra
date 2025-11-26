import { useRef, useCallback, useEffect } from 'react'
import {
  createAnalyticsState,
  startSession,
  endSession,
  recordQualitySwitch,
  recordStallEvent,
  recordPlayTime,
  shouldFlush,
  getEventsToFlush,
  requeueEvents,
  flushEvents,
  flushEventsBeacon,
  DEFAULT_ANALYTICS_CONFIG,
} from '../analytics/PlaybackAnalytics'
import type {
  PlaybackAnalyticsState,
  PlaybackAnalyticsConfig,
  QualitySwitchEvent,
} from '../analytics/PlaybackAnalytics'

export interface UsePlaybackAnalyticsOptions {
  enabled?: boolean
  config?: Partial<PlaybackAnalyticsConfig>
}

export interface UsePlaybackAnalyticsReturn {
  startSession: (mediaId: number) => string
  endSession: () => void
  recordQualitySwitch: (
    fromQuality: string | null,
    toQuality: string,
    reason: QualitySwitchEvent['switchReason'],
    positionSeconds: number,
    networkSpeedMbps: number | null,
    bufferSeconds: number | null,
    causedStall?: boolean
  ) => void
  recordStall: (durationMs: number) => void
  recordPlayTime: (durationMs: number) => void
}

/**
 * React hook for playback analytics collection
 *
 * Usage:
 * ```tsx
 * const analytics = usePlaybackAnalytics({ enabled: true })
 *
 * // When playback starts
 * const sessionId = analytics.startSession(mediaId)
 *
 * // When quality changes
 * analytics.recordQualitySwitch(
 *   '720p', '1080p', 'auto_bandwidth',
 *   video.currentTime, networkStats.averageThroughputMbps, bufferLength
 * )
 *
 * // When playback ends
 * analytics.endSession()
 * ```
 */
export const usePlaybackAnalytics = (
  options: UsePlaybackAnalyticsOptions = {}
): UsePlaybackAnalyticsReturn => {
  const { enabled = true, config } = options

  const stateRef = useRef<PlaybackAnalyticsState>(createAnalyticsState())
  const configRef = useRef<PlaybackAnalyticsConfig>({ ...DEFAULT_ANALYTICS_CONFIG, ...config })
  const flushIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Flush events periodically and on unmount
  useEffect(() => {
    if (!enabled) {
      return
    }

    // Capture config at effect setup time for cleanup
    const currentConfig = configRef.current

    const flush = async () => {
      if (stateRef.current.eventQueue.length === 0) {
        return
      }

      const { state, events, session } = getEventsToFlush(stateRef.current)
      stateRef.current = state

      const success = await flushEvents(events, session, currentConfig)
      if (!success) {
        stateRef.current = requeueEvents(stateRef.current, events)
      }
    }

    flushIntervalRef.current = setInterval(flush, currentConfig.flushIntervalMs)

    return () => {
      if (flushIntervalRef.current) {
        clearInterval(flushIntervalRef.current)
      }
      // Final flush on unmount using sendBeacon
      if (stateRef.current.eventQueue.length > 0) {
        const { events, session } = getEventsToFlush(stateRef.current)
        flushEventsBeacon(events, session, currentConfig)
      }
    }
  }, [enabled])

  const handleStartSession = useCallback((mediaId: number): string => {
    const result = startSession(stateRef.current, mediaId)
    stateRef.current = result.state
    return result.sessionId
  }, [])

  const handleEndSession = useCallback(() => {
    stateRef.current = endSession(stateRef.current)
    // Flush remaining events
    const { events, session } = getEventsToFlush(stateRef.current)
    if (events.length > 0) {
      flushEvents(events, session, configRef.current)
    }
  }, [])

  const handleRecordQualitySwitch = useCallback(
    (
      fromQuality: string | null,
      toQuality: string,
      reason: QualitySwitchEvent['switchReason'],
      positionSeconds: number,
      networkSpeedMbps: number | null,
      bufferSeconds: number | null,
      causedStall = false
    ) => {
      stateRef.current = recordQualitySwitch(
        stateRef.current,
        fromQuality,
        toQuality,
        reason,
        positionSeconds,
        networkSpeedMbps,
        bufferSeconds,
        causedStall
      )

      // Check if we should flush based on batch size
      if (shouldFlush(stateRef.current, configRef.current)) {
        const { state, events, session } = getEventsToFlush(stateRef.current)
        stateRef.current = state
        flushEvents(events, session, configRef.current)
      }
    },
    []
  )

  const handleRecordStall = useCallback((durationMs: number) => {
    stateRef.current = recordStallEvent(stateRef.current, durationMs)
  }, [])

  const handleRecordPlayTime = useCallback((durationMs: number) => {
    stateRef.current = recordPlayTime(stateRef.current, durationMs)
  }, [])

  return {
    startSession: handleStartSession,
    endSession: handleEndSession,
    recordQualitySwitch: handleRecordQualitySwitch,
    recordStall: handleRecordStall,
    recordPlayTime: handleRecordPlayTime,
  }
}

export default usePlaybackAnalytics
