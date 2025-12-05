/**
 * useWindowedFetching Hook
 * Generic hook for progressive windowed data fetching with two-phase loading.
 * Used by both text and PGS subtitle renderers.
 *
 * Features:
 * - Two-phase loading: micro-window for immediate display, full window in background
 * - Automatic retry with exponential backoff on failures
 * - Error tracking for UI feedback
 */

import { useEffect, useRef, useState, useCallback } from 'react'

// Window timing constants (shared by all subtitle types)
export const MICRO_WINDOW_MS = 5 * 1000 // 5 seconds for immediate display
export const FULL_WINDOW_MS = 2 * 60 * 1000 // 2 minutes
export const PREFETCH_THRESHOLD_MS = 60 * 1000 // Start prefetching 60s before window end

// Retry configuration
const RETRY_CONFIG = {
  maxRetries: 3,
  initialDelayMs: 1000,
  maxDelayMs: 8000,
  // Micro-windows use faster retries since they're for immediate display
  microMaxRetries: 2,
  microInitialDelayMs: 500,
}

/**
 * Error information for a failed fetch operation.
 */
export interface FetchError {
  windowKey: string
  error: Error
  timestamp: number
  retryCount: number
}

export interface WindowedFetchingOptions<T> {
  /** Unique key for this data source (changes trigger reset) */
  sourceKey: string | null
  /** Function to fetch a time window of data */
  fetchWindow: (startMs: number, endMs: number, signal: AbortSignal) => Promise<T[]>
  /** Function to get unique key from an item (for deduplication) */
  getItemKey: (item: T) => number | string
  /** Initial playback time in milliseconds */
  initialTimeMs: number
  /** Whether windowed fetching is enabled (false = fetch full file) */
  windowedEnabled: boolean
  /** Optional function to fetch full data when windowed is disabled */
  fetchFull?: (signal: AbortSignal) => Promise<T[]>
}

export interface WindowedFetchingReturn<T> {
  /** Current items (merged from all fetched windows) */
  items: T[]
  /** Ref to access items in animation loop without triggering re-renders */
  itemsRef: React.RefObject<T[]>
  /** Ensure a window is loaded (call from animation loop for seeks/prefetch) */
  ensureWindowLoaded: (windowStart: number, actualTimeMs?: number) => void
  /** Calculate which window a timestamp falls into */
  getWindowStart: (timeMs: number) => number
  /** Whether full data has been fetched (for non-windowed mode) */
  isFullyLoaded: boolean
  /** Abort controller ref for cancellation */
  abortControllerRef: React.RefObject<AbortController | null>
  /** Set of fetched window starts (for checking if window is loaded) */
  fetchedWindowsRef: React.RefObject<Set<number>>
  /** Current fetch errors (cleared on successful retry or source change) */
  errors: FetchError[]
  /** Whether any fetch is currently in progress */
  isLoading: boolean
  /** Clear all errors (useful after user acknowledges them) */
  clearErrors: () => void
}

/**
 * Calculate delay for exponential backoff.
 */
const getBackoffDelay = (retryCount: number, initialDelay: number, maxDelay: number): number => {
  const delay = initialDelay * Math.pow(2, retryCount)
  // Add jitter (±25%) to prevent thundering herd
  const jitter = delay * 0.25 * (Math.random() * 2 - 1)
  return Math.min(delay + jitter, maxDelay)
}

export const useWindowedFetching = <T>({
  sourceKey,
  fetchWindow,
  getItemKey,
  initialTimeMs,
  windowedEnabled,
  fetchFull,
}: WindowedFetchingOptions<T>): WindowedFetchingReturn<T> => {
  const [items, setItems] = useState<T[]>([])
  const [errors, setErrors] = useState<FetchError[]>([])
  const [loadingCount, setLoadingCount] = useState(0)

  const itemsRef = useRef<T[]>([])
  const fetchedWindowsRef = useRef<Set<number>>(new Set())
  const fetchedMicroWindowsRef = useRef<Set<string>>(new Set())
  const pendingFetchesRef = useRef<Set<string>>(new Set())
  const fullFetchCompleteRef = useRef<boolean>(false)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Store callbacks in refs to avoid dependency cycles
  const fetchWindowRef = useRef(fetchWindow)
  const fetchFullRef = useRef(fetchFull)
  const getItemKeyRef = useRef(getItemKey)

  // Update refs when callbacks change
  useEffect(() => {
    fetchWindowRef.current = fetchWindow
  }, [fetchWindow])

  useEffect(() => {
    fetchFullRef.current = fetchFull
  }, [fetchFull])

  useEffect(() => {
    getItemKeyRef.current = getItemKey
  }, [getItemKey])

  // Keep itemsRef in sync
  useEffect(() => {
    itemsRef.current = items
  }, [items])

  const getWindowStart = useCallback((timeMs: number): number => {
    return Math.floor(timeMs / FULL_WINDOW_MS) * FULL_WINDOW_MS
  }, [])

  // Clear all errors
  const clearErrors = useCallback(() => {
    setErrors([])
  }, [])

  // Add an error to the list
  const addError = useCallback((windowKey: string, error: Error, retryCount: number) => {
    setErrors((prev) => {
      // Remove any existing error for this window
      const filtered = prev.filter((e) => e.windowKey !== windowKey)
      return [...filtered, { windowKey, error, timestamp: Date.now(), retryCount }]
    })
  }, [])

  // Remove an error (on successful retry)
  const removeError = useCallback((windowKey: string) => {
    setErrors((prev) => prev.filter((e) => e.windowKey !== windowKey))
  }, [])

  // Merge new items, avoiding duplicates
  const mergeItems = useCallback((newItems: T[]) => {
    if (newItems.length === 0) { return }
    setItems((prev) => {
      const getKey = getItemKeyRef.current
      const existingKeys = new Set(prev.map(getKey))
      const uniqueNew = newItems.filter((item) => !existingKeys.has(getKey(item)))
      if (uniqueNew.length === 0) { return prev }
      const merged = [...prev, ...uniqueNew]
      // Sort by key (assumes key is sortable - works for timestamps)
      return merged.sort((a, b) => {
        const keyA = getKey(a)
        const keyB = getKey(b)
        return keyA < keyB ? -1 : keyA > keyB ? 1 : 0
      })
    })
  }, [])

  // Fetch with retry logic - stable callback that uses refs
  const fetchWithRetry = useCallback((
    windowKey: string,
    startMs: number,
    endMs: number,
    signal: AbortSignal,
    isMicro: boolean,
    retryCount = 0
  ): void => {
    const maxRetries = isMicro ? RETRY_CONFIG.microMaxRetries : RETRY_CONFIG.maxRetries
    const initialDelay = isMicro ? RETRY_CONFIG.microInitialDelayMs : RETRY_CONFIG.initialDelayMs

    setLoadingCount((c) => c + 1)

    fetchWindowRef.current(startMs, endMs, signal)
      .then((data) => {
        if (!signal.aborted) {
          mergeItems(data)
          removeError(windowKey)
          pendingFetchesRef.current.delete(windowKey)
        }
      })
      .catch((err) => {
        if (err instanceof Error && err.name === 'AbortError') {
          pendingFetchesRef.current.delete(windowKey)
          return
        }

        const error = err instanceof Error ? err : new Error(String(err))

        if (retryCount < maxRetries && !signal.aborted) {
          // Schedule retry with backoff
          const delay = getBackoffDelay(retryCount, initialDelay, RETRY_CONFIG.maxDelayMs)

          setTimeout(() => {
            if (!signal.aborted && pendingFetchesRef.current.has(windowKey)) {
              fetchWithRetry(windowKey, startMs, endMs, signal, isMicro, retryCount + 1)
            }
          }, delay)
        } else if (!signal.aborted) {
          // Max retries exceeded - record permanent error
          addError(windowKey, error, retryCount)
          pendingFetchesRef.current.delete(windowKey)
          console.error(`Failed to fetch ${isMicro ? 'micro-' : ''}window after ${retryCount} retries:`, error)
        }
      })
      .finally(() => {
        setLoadingCount((c) => Math.max(0, c - 1))
      })
  }, [mergeItems, addError, removeError])

  // Two-phase window loading with retry support - stable callback
  const ensureWindowLoaded = useCallback((windowStart: number, actualTimeMs?: number) => {
    if (fullFetchCompleteRef.current) { return }
    if (!abortControllerRef.current) { return }

    const signal = abortControllerRef.current.signal
    const windowEnd = windowStart + FULL_WINDOW_MS
    const microStart = actualTimeMs !== undefined
      ? Math.floor(actualTimeMs / 1000) * 1000
      : windowStart
    const microKey = `micro-${microStart}`
    const fullKey = `full-${windowStart}`

    // Phase 1: Micro-window for immediate display
    if (!fetchedMicroWindowsRef.current.has(microKey) && !pendingFetchesRef.current.has(microKey)) {
      fetchedMicroWindowsRef.current.add(microKey)
      pendingFetchesRef.current.add(microKey)
      const microEnd = microStart + MICRO_WINDOW_MS
      fetchWithRetry(microKey, microStart, microEnd, signal, true)
    }

    // Phase 2: Full window in background
    if (!fetchedWindowsRef.current.has(windowStart) && !pendingFetchesRef.current.has(fullKey)) {
      fetchedWindowsRef.current.add(windowStart)
      pendingFetchesRef.current.add(fullKey)
      fetchWithRetry(fullKey, windowStart, windowEnd, signal, false)
    }
  }, [fetchWithRetry])

  // Fetch full data with retry logic (for non-windowed mode)
  const fetchFullWithRetry = useCallback((
    signal: AbortSignal,
    retryCount = 0
  ): void => {
    const currentFetchFull = fetchFullRef.current
    if (!currentFetchFull) { return }

    const fullKey = 'full-file'
    setLoadingCount((c) => c + 1)

    currentFetchFull(signal)
      .then((data) => {
        if (!signal.aborted) {
          setItems(data)
          fullFetchCompleteRef.current = true
          removeError(fullKey)
        }
      })
      .catch((err) => {
        if (err instanceof Error && err.name === 'AbortError') { return }

        const error = err instanceof Error ? err : new Error(String(err))

        if (retryCount < RETRY_CONFIG.maxRetries && !signal.aborted) {
          const delay = getBackoffDelay(retryCount, RETRY_CONFIG.initialDelayMs, RETRY_CONFIG.maxDelayMs)

          setTimeout(() => {
            if (!signal.aborted) {
              fetchFullWithRetry(signal, retryCount + 1)
            }
          }, delay)
        } else if (!signal.aborted) {
          addError(fullKey, error, retryCount)
          console.error(`Failed to fetch full data after ${retryCount} retries:`, error)
        }
      })
      .finally(() => {
        setLoadingCount((c) => Math.max(0, c - 1))
      })
  }, [addError, removeError])

  // Reset and fetch initial data when source changes
  // Only depends on stable values, not on callbacks
  useEffect(() => {
    if (sourceKey === null) {
      setItems([])
      setErrors([])
      fetchedWindowsRef.current.clear()
      fetchedMicroWindowsRef.current.clear()
      pendingFetchesRef.current.clear()
      fullFetchCompleteRef.current = false
      return
    }

    // Abort previous requests
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }

    setItems([])
    setErrors([])
    fetchedWindowsRef.current.clear()
    fetchedMicroWindowsRef.current.clear()
    pendingFetchesRef.current.clear()
    fullFetchCompleteRef.current = false

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    if (windowedEnabled) {
      const initialWindow = getWindowStart(initialTimeMs)
      // Defer to next tick so refs are set
      setTimeout(() => {
        ensureWindowLoaded(initialWindow, initialTimeMs)
      }, 0)
    } else {
      fetchFullWithRetry(abortController.signal)
    }

    return () => {
      abortController.abort()
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceKey, windowedEnabled, initialTimeMs])

  return {
    items,
    itemsRef,
    ensureWindowLoaded,
    getWindowStart,
    isFullyLoaded: fullFetchCompleteRef.current,
    abortControllerRef,
    fetchedWindowsRef,
    errors,
    isLoading: loadingCount > 0,
    clearErrors,
  }
}

/**
 * Hook for the animation loop that updates current item and handles prefetching.
 * Separated to allow different update logic for text vs bitmap subtitles.
 */
export interface UseSubtitleAnimationOptions<T> {
  videoRef: React.RefObject<HTMLVideoElement | null>
  streamOffsetRef: React.RefObject<number>
  subtitleDelay: number
  itemsRef: React.RefObject<T[]>
  /** Find active item at given time (seconds) */
  findActiveItem: (items: T[], timeSeconds: number) => T | null
  /** Called when active item changes */
  onActiveItemChange: (item: T | null) => void
  /** Optional: called each frame with current time in ms (for prefetching) */
  onFrame?: (timeMs: number) => void
  /** Whether this animation loop is enabled */
  enabled: boolean
}

export const useSubtitleAnimation = <T>({
  videoRef,
  streamOffsetRef,
  subtitleDelay,
  itemsRef,
  findActiveItem,
  onActiveItemChange,
  onFrame,
  enabled,
}: UseSubtitleAnimationOptions<T>) => {
  const lastItemRef = useRef<T | null>(null)

  // Store callbacks in refs to avoid re-running effect
  const findActiveItemRef = useRef(findActiveItem)
  const onActiveItemChangeRef = useRef(onActiveItemChange)
  const onFrameRef = useRef(onFrame)

  useEffect(() => {
    findActiveItemRef.current = findActiveItem
  }, [findActiveItem])

  useEffect(() => {
    onActiveItemChangeRef.current = onActiveItemChange
  }, [onActiveItemChange])

  useEffect(() => {
    onFrameRef.current = onFrame
  }, [onFrame])

  useEffect(() => {
    const video = videoRef.current
    if (!video || !enabled) { return }

    let animationId: number

    const update = () => {
      // Base time for window fetching (no delay applied)
      const baseTime = video.currentTime + (streamOffsetRef.current || 0)
      const baseTimeMs = baseTime * 1000

      // Time for subtitle display (with delay applied)
      const displayTime = baseTime - subtitleDelay

      // Use base time for prefetching windows
      onFrameRef.current?.(baseTimeMs)

      const currentItems = itemsRef.current
      if (currentItems.length > 0) {
        // Use display time for finding active subtitle
        const activeItem = findActiveItemRef.current(currentItems, displayTime)
        if (activeItem !== lastItemRef.current) {
          lastItemRef.current = activeItem
          onActiveItemChangeRef.current(activeItem)
        }
      }

      animationId = requestAnimationFrame(update)
    }

    animationId = requestAnimationFrame(update)
    return () => cancelAnimationFrame(animationId)
  }, [videoRef, streamOffsetRef, subtitleDelay, itemsRef, enabled])
}
