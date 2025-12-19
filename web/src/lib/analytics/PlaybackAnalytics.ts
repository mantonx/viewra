/**
 * PlaybackAnalytics - Collects playback quality metrics
 *
 * Used for debugging, identifying problematic content, and optimizing transcoding.
 */

import { detectDeviceType, getConnectionType } from '@/lib/capabilities'
import { authFetch } from '@/lib/utils/authFetch'

export interface QualitySwitchEvent {
  mediaId: number
  sessionId: string
  fromQuality: string | null
  toQuality: string
  switchReason: 'auto_bandwidth' | 'auto_buffer' | 'auto_stall' | 'user_manual' | 'initial'
  positionSeconds: number
  networkSpeedMbps: number | null
  bufferSeconds: number | null
  causedStall: boolean
  deviceType: string
  connectionType: string
  timestamp: number
}

export interface PlaybackSession {
  sessionId: string
  mediaId: number
  startTime: number
  endTime: number | null
  totalPlayTime: number
  totalBufferTime: number
  stallCount: number
  qualitySwitchCount: number
  averageQuality: string
  deviceType: string
  connectionType: string
  startupTimeMs: number | null
}

export interface PlaybackAnalyticsConfig {
  enabled: boolean
  batchSize: number
  flushIntervalMs: number
  endpoint: string
}

export interface PlaybackAnalyticsState {
  eventQueue: QualitySwitchEvent[]
  currentSession: PlaybackSession | null
}

export const DEFAULT_ANALYTICS_CONFIG: PlaybackAnalyticsConfig = {
  enabled: true,
  batchSize: 10,
  flushIntervalMs: 30000,
  endpoint: '/api/analytics/playback',
}

/**
 * Create initial analytics state
 */
export const createAnalyticsState = (): PlaybackAnalyticsState => ({
  eventQueue: [],
  currentSession: null,
})

/**
 * Start a new playback session
 * @param state Current analytics state
 * @param mediaId Media ID being played
 * @param externalSessionId Optional session ID from backend (for correlation with transcode sessions)
 */
export const startSession = (
  state: PlaybackAnalyticsState,
  mediaId: number,
  externalSessionId?: string
): { state: PlaybackAnalyticsState; sessionId: string } => {
  // Use backend-provided session ID for transcode correlation, or generate a local one
  const sessionId = externalSessionId || `${mediaId}-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

  const session: PlaybackSession = {
    sessionId,
    mediaId,
    startTime: Date.now(),
    endTime: null,
    totalPlayTime: 0,
    totalBufferTime: 0,
    stallCount: 0,
    qualitySwitchCount: 0,
    averageQuality: '',
    deviceType: detectDeviceType(),
    connectionType: getConnectionType(),
    startupTimeMs: null,
  }

  return {
    state: { ...state, currentSession: session },
    sessionId,
  }
}

/**
 * Update the session ID (used when backend session ID arrives after session start)
 */
export const updateSessionId = (
  state: PlaybackAnalyticsState,
  newSessionId: string
): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  return {
    ...state,
    currentSession: {
      ...state.currentSession,
      sessionId: newSessionId,
    },
  }
}

/**
 * End the current session
 */
export const endSession = (state: PlaybackAnalyticsState): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  return {
    ...state,
    currentSession: {
      ...state.currentSession,
      endTime: Date.now(),
    },
  }
}

/**
 * Record a quality switch event
 */
export const recordQualitySwitch = (
  state: PlaybackAnalyticsState,
  fromQuality: string | null,
  toQuality: string,
  reason: QualitySwitchEvent['switchReason'],
  positionSeconds: number,
  networkSpeedMbps: number | null,
  bufferSeconds: number | null,
  causedStall = false
): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  const event: QualitySwitchEvent = {
    mediaId: state.currentSession.mediaId,
    sessionId: state.currentSession.sessionId,
    fromQuality,
    toQuality,
    switchReason: reason,
    positionSeconds,
    networkSpeedMbps,
    bufferSeconds,
    causedStall,
    deviceType: state.currentSession.deviceType,
    connectionType: getConnectionType(),
    timestamp: Date.now(),
  }

  return {
    ...state,
    eventQueue: [...state.eventQueue, event],
    currentSession: {
      ...state.currentSession,
      qualitySwitchCount: state.currentSession.qualitySwitchCount + 1,
    },
  }
}

/**
 * Record a stall event
 */
export const recordStallEvent = (
  state: PlaybackAnalyticsState,
  durationMs: number
): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  return {
    ...state,
    currentSession: {
      ...state.currentSession,
      stallCount: state.currentSession.stallCount + 1,
      totalBufferTime: state.currentSession.totalBufferTime + durationMs,
    },
  }
}

/**
 * Record play time
 */
export const recordPlayTime = (
  state: PlaybackAnalyticsState,
  durationMs: number
): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  return {
    ...state,
    currentSession: {
      ...state.currentSession,
      totalPlayTime: state.currentSession.totalPlayTime + durationMs,
    },
  }
}

/**
 * Record startup time (time from play request to first frame)
 */
export const recordStartupTime = (
  state: PlaybackAnalyticsState,
  startupTimeMs: number
): PlaybackAnalyticsState => {
  if (!state.currentSession) {
    return state
  }

  return {
    ...state,
    currentSession: {
      ...state.currentSession,
      startupTimeMs,
    },
  }
}

/**
 * Check if events should be flushed based on batch size
 */
export const shouldFlush = (
  state: PlaybackAnalyticsState,
  config: PlaybackAnalyticsConfig = DEFAULT_ANALYTICS_CONFIG
): boolean => {
  return state.eventQueue.length >= config.batchSize
}

/**
 * Get events to flush and clear the queue
 */
export const getEventsToFlush = (
  state: PlaybackAnalyticsState
): { state: PlaybackAnalyticsState; events: QualitySwitchEvent[]; session: PlaybackSession | null } => {
  return {
    state: { ...state, eventQueue: [] },
    events: state.eventQueue,
    session: state.currentSession,
  }
}

/**
 * Re-queue events on flush failure (with limit)
 */
export const requeueEvents = (
  state: PlaybackAnalyticsState,
  events: QualitySwitchEvent[],
  maxQueueSize = 100
): PlaybackAnalyticsState => {
  if (state.eventQueue.length >= maxQueueSize) {
    return state
  }

  const availableSpace = maxQueueSize - state.eventQueue.length
  const eventsToRequeue = events.slice(0, availableSpace)

  return {
    ...state,
    eventQueue: [...eventsToRequeue, ...state.eventQueue],
  }
}

/**
 * Flush events to the server
 */
export const flushEvents = async (
  events: QualitySwitchEvent[],
  session: PlaybackSession | null,
  config: PlaybackAnalyticsConfig = DEFAULT_ANALYTICS_CONFIG
): Promise<boolean> => {
  // Send if we have events OR if we have a session (session data is valuable even without events)
  if ((events.length === 0 && !session) || !config.enabled) {
    return true
  }

  try {
    const response = await authFetch(config.endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session, events }),
    })
    return response.ok
  } catch {
    return false
  }
}

/**
 * Flush events using sendBeacon (for page unload)
 */
export const flushEventsBeacon = (
  events: QualitySwitchEvent[],
  session: PlaybackSession | null,
  config: PlaybackAnalyticsConfig = DEFAULT_ANALYTICS_CONFIG
): boolean => {
  // Send if we have events OR if we have a session (session data is valuable even without events)
  if ((events.length === 0 && !session) || !config.enabled) {
    return true
  }

  return navigator.sendBeacon(
    config.endpoint,
    JSON.stringify({ session, events })
  )
}
