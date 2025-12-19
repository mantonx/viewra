import { useRef, useCallback, useEffect } from 'react'
import {
  createAnalyticsState,
  startSession,
  endSession,
  updateSessionId,
  recordQualitySwitch,
  recordStallEvent,
  recordPlayTime,
  recordStartupTime,
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
  /** Start a new playback session. Pass externalSessionId for backend correlation. */
  startSession: (mediaId: number, externalSessionId?: string) => string
  /** Update session ID when backend ID arrives after session start */
  updateSessionId: (newSessionId: string) => void
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
  recordStartupTime: (startupTimeMs: number) => void
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
      // Flush if we have events OR an active session (session data is valuable even without events)
      if (stateRef.current.eventQueue.length === 0 && !stateRef.current.currentSession) {
        console.log('[Analytics] Skipping flush - no events and no session')
        return
      }

      console.log('[Analytics] Flushing', {
        eventCount: stateRef.current.eventQueue.length,
        hasSession: !!stateRef.current.currentSession,
        sessionId: stateRef.current.currentSession?.sessionId,
      })

      const { state, events, session } = getEventsToFlush(stateRef.current)
      stateRef.current = state

      const success = await flushEvents(events, session, currentConfig)
      console.log('[Analytics] Flush result:', success)
      if (!success) {
        stateRef.current = requeueEvents(stateRef.current, events)
      }
    }

    flushIntervalRef.current = setInterval(flush, currentConfig.flushIntervalMs)

    return () => {
      if (flushIntervalRef.current) {
        clearInterval(flushIntervalRef.current)
      }
      // Final flush on unmount using sendBeacon (send if events OR active session)
      if (stateRef.current.eventQueue.length > 0 || stateRef.current.currentSession) {
        const { events, session } = getEventsToFlush(stateRef.current)
        flushEventsBeacon(events, session, currentConfig)
      }
    }
  }, [enabled])

  const handleStartSession = useCallback((mediaId: number, externalSessionId?: string): string => {
    console.log('[Analytics] Starting session', { mediaId, externalSessionId })
    const result = startSession(stateRef.current, mediaId, externalSessionId)
    stateRef.current = result.state
    console.log('[Analytics] Session started', { sessionId: result.sessionId })
    return result.sessionId
  }, [])

  const handleUpdateSessionId = useCallback((newSessionId: string) => {
    console.log('[Analytics] Updating session ID', { newSessionId })
    stateRef.current = updateSessionId(stateRef.current, newSessionId)
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

  const handleRecordStartupTime = useCallback((startupTimeMs: number) => {
    stateRef.current = recordStartupTime(stateRef.current, startupTimeMs)
  }, [])

  return {
    startSession: handleStartSession,
    updateSessionId: handleUpdateSessionId,
    endSession: handleEndSession,
    recordQualitySwitch: handleRecordQualitySwitch,
    recordStall: handleRecordStall,
    recordPlayTime: handleRecordPlayTime,
    recordStartupTime: handleRecordStartupTime,
  }
}

export default usePlaybackAnalytics
