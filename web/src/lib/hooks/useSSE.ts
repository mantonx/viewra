import { useState, useEffect, useRef, useCallback } from 'react'
import { API_BASE_URL } from '@/lib/config'
import { getStoredAccessToken } from '@/lib/utils/authFetch'

export type SSEConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error'

export interface UseSSEOptions<T> {
  /** Enable/disable the SSE connection */
  enabled?: boolean
  /** Called when an event is received */
  onEvent?: (event: T) => void
  /** Called when connection state changes */
  onStateChange?: (state: SSEConnectionState) => void
  /** Called on error */
  onError?: (error: Error) => void
  /** Reconnect delay in ms (default: 3000) */
  reconnectDelay?: number
  /** Max reconnect attempts (default: 5, 0 = infinite) */
  maxReconnectAttempts?: number
  /** Event types to listen for (default: all) */
  eventTypes?: string[]
}

export interface UseSSEReturn<T> {
  /** Current connection state */
  connectionState: SSEConnectionState
  /** Last received event data */
  lastEvent: T | null
  /** Error if any */
  error: Error | null
  /** Manually reconnect */
  reconnect: () => void
  /** Manually disconnect */
  disconnect: () => void
}

/**
 * Generic hook for authenticated Server-Sent Events (SSE) connections.
 *
 * Uses fetch-based streaming to support Authorization headers (EventSource
 * doesn't support custom headers).
 *
 * @example
 * ```tsx
 * const { connectionState, lastEvent } = useSSE<ScanProgressEvent>(
 *   '/api/libraries/1/scan/stream',
 *   {
 *     enabled: true,
 *     onEvent: (event) => console.log('Received:', event),
 *   }
 * )
 * ```
 */
export const useSSE = <T = unknown>(
  url: string,
  options: UseSSEOptions<T> = {}
): UseSSEReturn<T> => {
  const {
    enabled = true,
    onEvent,
    onStateChange,
    onError,
    reconnectDelay = 3000,
    maxReconnectAttempts = 5,
    eventTypes,
  } = options

  const [connectionState, setConnectionState] = useState<SSEConnectionState>('disconnected')
  const [lastEvent, setLastEvent] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)

  const abortControllerRef = useRef<AbortController | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isManualDisconnectRef = useRef(false)

  // Store callbacks in refs to avoid recreating connect() when callbacks change
  const onEventRef = useRef(onEvent)
  const onErrorRef = useRef(onError)
  const onStateChangeRef = useRef(onStateChange)
  const eventTypesRef = useRef(eventTypes)
  const connectRef = useRef<() => void>(() => {})

  // Keep refs updated with latest callbacks
  useEffect(() => {
    onEventRef.current = onEvent
    onErrorRef.current = onError
    onStateChangeRef.current = onStateChange
    eventTypesRef.current = eventTypes
  }, [onEvent, onError, onStateChange, eventTypes])

  const updateState = useCallback((newState: SSEConnectionState) => {
    setConnectionState(newState)
    onStateChangeRef.current?.(newState)
  }, [])

  // scheduleReconnect uses connectRef to avoid circular dependency
  const scheduleReconnect = useCallback(() => {
    if (maxReconnectAttempts > 0 && reconnectAttemptsRef.current >= maxReconnectAttempts) {
      return
    }

    reconnectAttemptsRef.current++

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }

    reconnectTimeoutRef.current = setTimeout(() => {
      if (!isManualDisconnectRef.current) {
        connectRef.current()
      }
    }, reconnectDelay)
  }, [maxReconnectAttempts, reconnectDelay])

  const connect = useCallback(async () => {
    // Clean up any existing connection
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }

    const token = getStoredAccessToken()
    if (!token) {
      const authError = new Error('No authentication token available')
      setError(authError)
      onErrorRef.current?.(authError)
      updateState('error')
      return
    }

    isManualDisconnectRef.current = false
    updateState('connecting')
    setError(null)

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    try {
      const fullUrl = url.startsWith('http') ? url : `${API_BASE_URL}${url}`

      const response = await fetch(fullUrl, {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: 'text/event-stream',
          'Cache-Control': 'no-cache',
        },
        signal: abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`SSE connection failed: ${response.status} ${response.statusText}`)
      }

      if (!response.body) {
        throw new Error('Response body is null')
      }

      updateState('connected')
      reconnectAttemptsRef.current = 0

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      // Read the stream
      while (true) {
        const { done, value } = await reader.read()

        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })

        // Parse SSE events from buffer
        const lines = buffer.split('\n')
        buffer = lines.pop() || '' // Keep incomplete line in buffer

        let currentEvent: { type?: string; data?: string } = {}

        for (const line of lines) {
          if (line.startsWith('event:')) {
            currentEvent.type = line.slice(6).trim()
          } else if (line.startsWith('data:')) {
            currentEvent.data = line.slice(5).trim()
          } else if (line === '' && currentEvent.data) {
            // Empty line signals end of event
            const eventTypesNow = eventTypesRef.current
            const shouldProcess =
              !eventTypesNow || !currentEvent.type || eventTypesNow.includes(currentEvent.type)

            if (shouldProcess && currentEvent.data) {
              try {
                const parsed = JSON.parse(currentEvent.data) as T
                setLastEvent(parsed)
                onEventRef.current?.(parsed)
              } catch {
                // Non-JSON data, pass as-is if T is string-compatible
                setLastEvent(currentEvent.data as T)
                onEventRef.current?.(currentEvent.data as T)
              }
            }
            currentEvent = {}
          }
        }
      }

      // Stream ended normally
      if (!isManualDisconnectRef.current) {
        updateState('disconnected')
        scheduleReconnect()
      }
    } catch (err) {
      if (abortController.signal.aborted) {
        // Intentional abort, don't treat as error
        return
      }

      const connectionError = err instanceof Error ? err : new Error(String(err))
      setError(connectionError)
      onErrorRef.current?.(connectionError)
      updateState('error')

      if (!isManualDisconnectRef.current) {
        scheduleReconnect()
      }
    }
  }, [url, updateState, scheduleReconnect])

  // Keep connectRef updated so scheduleReconnect can call latest connect
  useEffect(() => {
    connectRef.current = connect
  }, [connect])

  const disconnect = useCallback(() => {
    isManualDisconnectRef.current = true

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }

    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
    }

    updateState('disconnected')
  }, [updateState])

  const reconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0
    disconnect()
    // Small delay to ensure cleanup completes
    setTimeout(() => {
      isManualDisconnectRef.current = false
      connect()
    }, 100)
  }, [connect, disconnect])

  // Connect/disconnect based on enabled flag
  useEffect(() => {
    if (enabled) {
      connect()
    } else {
      disconnect()
    }

    return () => {
      disconnect()
    }
  }, [enabled, connect, disconnect])

  return {
    connectionState,
    lastEvent,
    error,
    reconnect,
    disconnect,
  }
}

export default useSSE
