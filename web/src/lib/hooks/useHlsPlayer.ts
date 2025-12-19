/**
 * useHlsPlayer Hook
 *
 * Manages HLS.js instance lifecycle, tracks, and error handling.
 *
 * Single-Quality Architecture:
 * - Backend picks optimal quality based on client capabilities (screen size, bandwidth)
 * - Master playlist contains only ONE variant (no HLS.js ABR decision)
 * - Quality changes trigger a full stream reload via URL rebuild
 * - Each quality change restarts FFmpeg from current position
 */

import { useEffect, useRef, useState, useCallback } from 'react'
import Hls from 'hls.js'
import { getAuthHeaders } from '@/lib/utils/authFetch'
import { logger } from '@/lib/utils/logger'
import { ensureVideoUnmuted } from '@/lib/utils/videoUtils'

// HLS configuration constants
// Optimized for on-demand transcoding where segments are generated progressively.
// Balance between fast startup and smooth seeking (segments may not be ready yet).
const HLS_CONFIG = {
  // Buffer settings: Allow more buffer for on-demand transcoding
  // This gives FFmpeg time to generate segments ahead of playback
  MAX_BUFFER_LENGTH: 6, // Buffer 3 segments (6s) before playback - allows headroom
  MAX_MAX_BUFFER_LENGTH: 30, // Allow larger buffer for seeking ahead
  MAX_BUFFER_SIZE: 60 * 1000 * 1000, // 60MB for 4K segments
  MAX_BUFFER_HOLE: 0.5, // Tighter sync to avoid jumps
  ENABLE_WORKER: true, // Enable for better performance
  LOW_LATENCY_MODE: false, // Disable for on-demand transcoding - we need buffer headroom
  DEBUG: false,
  BACK_BUFFER_LENGTH: 30, // Keep 30s back buffer for seeking backward
  HIGH_BUFFER_WATCHDOG_PERIOD: 2, // Less aggressive watchdog
  FRAG_LOADING_MAX_RETRY: 8, // More retries for slow segment generation
  FRAG_LOADING_MAX_RETRY_TIMEOUT: 64000,
  NUDGE_OFFSET: 0.1,
  NUDGE_MAX_RETRY: 10,
  // Start loading next segments earlier to stay ahead of playback
  START_FRAG_PREFETCH: true,
} as const

export interface QualityLevel {
  height: number
  bandwidth: number
  index?: number
  isOriginal?: boolean
}

export interface AudioTrack {
  id: number
  name: string
  language: string
}

export interface SubtitleTrack {
  id: number
  name: string
  language: string
}

export interface UseHlsPlayerOptions {
  videoRef: React.RefObject<HTMLVideoElement | null>
  streamUrl: string
  initialPosition: number
  isHlsStream: boolean
  onError: (error: string | null) => void
  onFragLoaded?: (bytes: number, durationMs: number) => void
}

export interface UseHlsPlayerReturn {
  hlsRef: React.RefObject<Hls | null>
  availableQualities: QualityLevel[]
  currentQuality: number | null
  currentBandwidth: number | null
  availableAudioTracks: AudioTrack[]
  currentAudioTrack: number
  availableSubtitleTracks: SubtitleTrack[]
  currentSubtitleTrack: number
  streamOffsetRef: React.RefObject<number>
  changeQuality: (height: number, bandwidth?: number) => number | null
  changeAudioTrack: (trackId: number) => void
  changeSubtitleTrack: (trackId: number) => void
  loadSource: (url: string) => void
}


// Initialize stream offset for progressive transcoding
// This sets the offset between video.currentTime (starts at 0) and actual media time
const initializeStreamOffset = (
  streamOffsetRef: { current: number },
  initialPosition: number
) => {
  // Always update the offset to match the new initial position
  // This handles both initial load and quality/audio track switches
  if (initialPosition > 0) {
    streamOffsetRef.current = initialPosition
  } else if (streamOffsetRef.current > 0) {
    // Reset offset when going back to start
    streamOffsetRef.current = 0
  }
}

/**
 * Find the best matching HLS level for a quality selection
 */
const findRecommendedLevel = (
  levels: Hls['levels'],
  height: number,
  bandwidth?: number
): number => {
  if (levels.length === 0) {return -1}

  // Find all levels at the requested height
  const levelsAtHeight = levels
    .map((level, index) => ({ level, index }))
    .filter((item) => item.level.height === height)

  if (levelsAtHeight.length > 0) {
    if (bandwidth) {
      // Try exact bandwidth match first
      const exactMatch = levelsAtHeight.find((item) => item.level.bitrate === bandwidth)
      if (exactMatch) {return exactMatch.index}

      // Find closest bandwidth at this height
      const sorted = [...levelsAtHeight].sort((a, b) =>
        Math.abs(a.level.bitrate - bandwidth) - Math.abs(b.level.bitrate - bandwidth)
      )
      return sorted[0].index
    }
    // No bandwidth specified - pick highest bitrate at this height
    const sorted = [...levelsAtHeight].sort((a, b) => b.level.bitrate - a.level.bitrate)
    return sorted[0].index
  }

  // No levels at requested height - find closest height
  const sortedLevels = levels
    .map((level, index) => ({
      level,
      index,
      diff: Math.abs(level.height - height),
    }))
    .filter((item) => item.level.height > 0)
    .sort((a, b) => {
      if (a.diff !== b.diff) {return a.diff - b.diff}
      return b.level.bitrate - a.level.bitrate
    })

  return sortedLevels.length > 0 ? sortedLevels[0].index : -1
}

export const useHlsPlayer = ({
  videoRef,
  streamUrl,
  initialPosition,
  isHlsStream,
  onError,
  onFragLoaded,
}: UseHlsPlayerOptions): UseHlsPlayerReturn => {
  const hlsRef = useRef<Hls | null>(null)
  const streamOffsetRef = useRef<number>(0)

  const [availableQualities, setAvailableQualities] = useState<QualityLevel[]>([])
  const [currentQuality, setCurrentQuality] = useState<number | null>(null)
  const [currentBandwidth, setCurrentBandwidth] = useState<number | null>(null)
  const [availableAudioTracks, setAvailableAudioTracks] = useState<AudioTrack[]>([])
  const [currentAudioTrack, setCurrentAudioTrack] = useState<number>(-1)
  const [availableSubtitleTracks, setAvailableSubtitleTracks] = useState<SubtitleTrack[]>([])
  const [currentSubtitleTrack, setCurrentSubtitleTrack] = useState<number>(-1)

  // Store callbacks in refs to avoid effect re-runs when callbacks change
  const onErrorRef = useRef(onError)
  onErrorRef.current = onError
  const onFragLoadedRef = useRef(onFragLoaded)
  onFragLoadedRef.current = onFragLoaded

  /**
   * Change quality level (legacy - not used in single-quality mode)
   *
   * In single-quality mode, quality changes happen at the URL level (triggering
   * a new FFmpeg session), not at the HLS.js level. This function remains for
   * potential future multi-variant playlist support.
   */
  const changeQuality = useCallback((height: number, bandwidth?: number): number | null => {
    const hls = hlsRef.current
    if (!hls) {return null}

    // Find and set specific level
    const levelIndex = findRecommendedLevel(hls.levels, height, bandwidth)

    if (levelIndex !== -1) {
      hls.currentLevel = levelIndex
      const selectedLevel = hls.levels[levelIndex]
      logger.info('[HLS] Level selected', {
        height: selectedLevel.height,
        bitrate: selectedLevel.bitrate,
        levelIndex,
      })
      return levelIndex
    }

    return null
  }, [])

  // Change audio track
  const changeAudioTrack = useCallback((trackId: number) => {
    const hls = hlsRef.current
    if (hls) {
      hls.audioTrack = trackId
      setCurrentAudioTrack(trackId)
    }
  }, [])

  // Change subtitle track (-1 = off)
  const changeSubtitleTrack = useCallback((trackId: number) => {
    const hls = hlsRef.current
    if (hls) {
      hls.subtitleTrack = trackId
      setCurrentSubtitleTrack(trackId)
    }
  }, [])

  // Load a new source URL
  const loadSource = useCallback((url: string) => {
    const hls = hlsRef.current
    if (hls) {
      hls.loadSource(url)
    }
  }, [])

  // Initialize HLS player
  useEffect(() => {
    const video = videoRef.current
    if (!video || !streamUrl) {return}

    // For direct streams, use native HTML5 video
    if (!isHlsStream) {
      video.src = streamUrl
      if (initialPosition > 0) {
        video.currentTime = initialPosition
      }
      video.muted = true
      video.play()
        .then(() => ensureVideoUnmuted(video))
        .catch(() => { /* Autoplay blocked */ })
      return
    }

    // Check for native HLS support (Safari/iOS)
    const canPlayHls = video.canPlayType('application/vnd.apple.mpegurl')
    if (canPlayHls) {
      video.src = streamUrl
      initializeStreamOffset(streamOffsetRef, initialPosition)
      video.muted = true
      video.play()
        .then(() => ensureVideoUnmuted(video))
        .catch(() => { /* Autoplay blocked */ })
      return
    }

    // Use hls.js for browsers without native HLS support
    if (!Hls.isSupported()) {
      onErrorRef.current('Browser does not support HLS streaming')
      return
    }

    // Get bandwidth estimate from Navigator Connection API for initial buffer sizing
    // In single-quality mode, this helps HLS.js with buffer decisions (not quality selection)
    const connection = (navigator as { connection?: { downlink?: number } }).connection
    let bandwidthEstimate = 20_000_000 // Default 20 Mbps (reasonable for modern connections)

    if (connection?.downlink && connection.downlink > 0) {
      // Navigator API gives Mbps, convert to bps for HLS.js
      bandwidthEstimate = connection.downlink * 1_000_000 * 1.2
      logger.info('[HLS] Bandwidth estimate from navigator.connection', {
        downlinkMbps: connection.downlink,
        estimate: bandwidthEstimate,
      })
    } else {
      logger.info('[HLS] Navigator connection API unavailable, using default', {
        estimate: bandwidthEstimate,
      })
    }

    // Create hls.js instance
    const hls = new Hls({
      debug: HLS_CONFIG.DEBUG,
      enableWorker: HLS_CONFIG.ENABLE_WORKER,
      lowLatencyMode: HLS_CONFIG.LOW_LATENCY_MODE,
      maxBufferLength: HLS_CONFIG.MAX_BUFFER_LENGTH,
      maxMaxBufferLength: HLS_CONFIG.MAX_MAX_BUFFER_LENGTH,
      maxBufferSize: HLS_CONFIG.MAX_BUFFER_SIZE,
      maxBufferHole: HLS_CONFIG.MAX_BUFFER_HOLE,
      backBufferLength: HLS_CONFIG.BACK_BUFFER_LENGTH,
      highBufferWatchdogPeriod: HLS_CONFIG.HIGH_BUFFER_WATCHDOG_PERIOD,
      fragLoadingMaxRetry: HLS_CONFIG.FRAG_LOADING_MAX_RETRY,
      fragLoadingMaxRetryTimeout: HLS_CONFIG.FRAG_LOADING_MAX_RETRY_TIMEOUT,
      nudgeOffset: HLS_CONFIG.NUDGE_OFFSET,
      nudgeMaxRetry: HLS_CONFIG.NUDGE_MAX_RETRY,
      startFragPrefetch: HLS_CONFIG.START_FRAG_PREFETCH,
      // Single-quality playback: Backend picks optimal quality, we just play it
      // startLevel=0 means play the first (and only) variant in the master playlist
      startLevel: 0,
      abrEwmaDefaultEstimate: bandwidthEstimate, // Helps with buffer decisions
      xhrSetup: (xhr, _url) => {
        const authHeaders = getAuthHeaders()
        if (authHeaders['Authorization']) {
          xhr.setRequestHeader('Authorization', authHeaders['Authorization'])
        }
      },
    })
    hlsRef.current = hls

    // Handle manifest parsed - extract quality levels for UI
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      const levels = hls.levels

      if (levels && levels.length > 0) {
        // Find the highest resolution available - this is the "original" (source) quality
        // since we filter out qualities higher than source in the backend
        const maxHeight = Math.max(...levels.map(l => l.height).filter(h => h > 0))

        // Build quality list for UI
        const qualities = levels
          .map((level, index) => {
            // Mark the highest resolution as "original" - it matches the source
            const isOriginal = level.height === maxHeight
            return {
              height: level.height,
              bandwidth: level.bitrate,
              index,
              isOriginal,
            }
          })
          .filter((q) => q.height > 0)
          .sort((a, b) => {
            if (b.height !== a.height) {return b.height - a.height}
            return b.bandwidth - a.bandwidth
          })

        setAvailableQualities(qualities)

        // Log the quality being played (single-quality mode - backend selected)
        const initialLevel = hls.startLevel >= 0 ? hls.startLevel : hls.firstLevel
        if (initialLevel >= 0 && levels[initialLevel]) {
          const level = levels[initialLevel]
          logger.info('[HLS] Playing quality', {
            levelIndex: initialLevel,
            height: level.height,
            bitrate: level.bitrate,
          })
        }
      }

      // Extract audio tracks
      const audioTracks = hls.audioTracks
      if (audioTracks && audioTracks.length > 0) {
        const tracks = audioTracks.map((track, index) => ({
          id: index,
          name: track.name || `Track ${index + 1}`,
          language: track.lang || 'Unknown',
        }))
        setAvailableAudioTracks(tracks)
        setCurrentAudioTrack(hls.audioTrack)
      }

      // Extract subtitle tracks
      const subtitleTracks = hls.subtitleTracks
      if (subtitleTracks && subtitleTracks.length > 0) {
        const tracks = subtitleTracks.map((track, index) => ({
          id: index,
          name: track.name || `Subtitle ${index + 1}`,
          language: track.lang || 'Unknown',
        }))
        setAvailableSubtitleTracks(tracks)
        hls.subtitleDisplay = true
        const defaultTrack = subtitleTracks.findIndex(t => t.default)
        if (defaultTrack >= 0) {
          hls.subtitleTrack = defaultTrack
          setCurrentSubtitleTrack(defaultTrack)
        }
      }

      initializeStreamOffset(streamOffsetRef, initialPosition)

      video.muted = true
      video.play()
        .then(() => ensureVideoUnmuted(video))
        .catch(() => { /* Autoplay blocked */ })
    })

    // Track quality level changes
    hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
      const level = hls.levels[data.level]
      if (level?.height) {
        setCurrentQuality(level.height)
        setCurrentBandwidth(level.bitrate || null)
        logger.debug('[HLS] Level switched', {
          height: level.height,
          bitrate: level.bitrate,
        })
      }
    })

    // Track audio track changes
    hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_event, data) => {
      setCurrentAudioTrack(data.id)
    })

    // Track subtitle track changes
    hls.on(Hls.Events.SUBTITLE_TRACK_SWITCH, (_event, data) => {
      setCurrentSubtitleTrack(data.id)
    })

    // Handle fragment loaded - for network sampling
    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      onErrorRef.current(null) // Clear any previous error
      if (onFragLoadedRef.current && data.frag?.stats) {
        const stats = data.frag.stats
        const bytes = stats.total || 0
        const durationMs = stats.loading?.end && stats.loading?.start
          ? stats.loading.end - stats.loading.start
          : 0
        if (bytes > 0 && durationMs > 0) {
          onFragLoadedRef.current(bytes, durationMs)
        }
      }
    })

    // Error handling with recovery limits
    let mediaErrorRecoveryAttempts = 0
    let streamReloadAttempts = 0
    const MAX_MEDIA_ERROR_RECOVERY = 3
    const MAX_STREAM_RELOAD = 1

    hls.on(Hls.Events.ERROR, (_event, data) => {
      // Log all errors for debugging
      logger.warn('[HLS] Error:', {
        type: data.type,
        details: data.details,
        fatal: data.fatal,
        url: data.url,
        reason: data.reason,
      })

      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            logger.error('[HLS] Fatal network error, retrying...', data.details)
            onErrorRef.current('Network issue: Retrying...')
            hls.startLoad()
            break
          case Hls.ErrorTypes.MEDIA_ERROR:
            mediaErrorRecoveryAttempts++
            if (mediaErrorRecoveryAttempts <= MAX_MEDIA_ERROR_RECOVERY) {
              logger.warn(`[HLS] Media error, recovery attempt ${mediaErrorRecoveryAttempts}/${MAX_MEDIA_ERROR_RECOVERY}`)
              onErrorRef.current(`Media error: Recovering (${mediaErrorRecoveryAttempts}/${MAX_MEDIA_ERROR_RECOVERY})...`)
              hls.recoverMediaError()
            } else if (streamReloadAttempts < MAX_STREAM_RELOAD) {
              // Try reloading the entire stream once as last resort
              streamReloadAttempts++
              mediaErrorRecoveryAttempts = 0
              logger.error('[HLS] Media error recovery failed, reloading stream')
              onErrorRef.current('Playback error: Reloading stream...')
              const currentUrl = hls.url
              if (currentUrl) {
                hls.loadSource(currentUrl)
              } else {
                hls.destroy()
                hlsRef.current = null
                onErrorRef.current('Playback failed - please refresh the page')
              }
            } else {
              // Give up - too many failures
              logger.error('[HLS] Media error recovery exhausted, giving up')
              onErrorRef.current('Playback failed - please try refreshing the page')
              hls.destroy()
              hlsRef.current = null
            }
            break
          default:
            logger.error('[HLS] Fatal error:', data.details)
            onErrorRef.current(`Playback error: ${data.details || 'Unknown error'}`)
            hls.destroy()
            hlsRef.current = null
            break
        }
      } else {
        // Non-fatal errors - just log them
        logger.debug('[HLS] Non-fatal error:', data.details)
      }
    })

    // Load source and attach to video
    hls.loadSource(streamUrl)
    hls.attachMedia(video)

    return () => {
      hls.destroy()
      hlsRef.current = null
    }
  // Note: onError and onFragLoaded are stored in refs to avoid triggering re-initialization

  }, [streamUrl, initialPosition, isHlsStream, videoRef])

  return {
    hlsRef,
    availableQualities,
    currentQuality,
    currentBandwidth,
    availableAudioTracks,
    currentAudioTrack,
    availableSubtitleTracks,
    currentSubtitleTrack,
    streamOffsetRef,
    changeQuality,
    changeAudioTrack,
    changeSubtitleTrack,
    loadSource,
  }
}
