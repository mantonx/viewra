/**
 * useHlsPlayer Hook
 * Manages HLS.js instance lifecycle, quality levels, audio tracks, and error handling.
 * Extracted from VideoPlayer to improve maintainability.
 */

import { useEffect, useRef, useState, useCallback } from 'react'
import Hls from 'hls.js'
import { getAuthHeaders } from '@/lib/utils/authFetch'
import { logger } from '@/lib/utils/logger'

// HLS configuration constants
const HLS_CONFIG = {
  MAX_BUFFER_LENGTH: 30,
  MAX_MAX_BUFFER_LENGTH: 60,
  MAX_BUFFER_SIZE: 100 * 1000 * 1000,
  MAX_BUFFER_HOLE: 2.0,
  ENABLE_WORKER: false,
  LOW_LATENCY_MODE: false,
  DEBUG: false,
  BACK_BUFFER_LENGTH: 90,
  HIGH_BUFFER_WATCHDOG_PERIOD: 1,
  FRAG_LOADING_MAX_RETRY: 6,
  FRAG_LOADING_MAX_RETRY_TIMEOUT: 64000,
  NUDGE_OFFSET: 0.1,
  NUDGE_MAX_RETRY: 10,
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
  qualityRecommendation?: {
    height?: number
    isReady?: boolean
  } | null
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
  setCurrentQuality: (quality: number | null) => void
  setCurrentBandwidth: (bandwidth: number | null) => void
  setCurrentAudioTrack: (trackId: number) => void
  changeQuality: (height: number, bandwidth?: number) => number | null
  changeAudioTrack: (trackId: number) => void
  changeSubtitleTrack: (trackId: number) => void
  loadSource: (url: string) => void
}

// Helper to ensure video is unmuted
const ensureVideoUnmuted = (video: HTMLVideoElement) => {
  video.muted = false
  video.volume = 1.0
}

// Initialize stream offset for progressive transcoding
const initializeStreamOffset = (
  streamOffsetRef: { current: number },
  initialPosition: number
) => {
  if (initialPosition > 0 && streamOffsetRef.current === 0) {
    streamOffsetRef.current = initialPosition
  }
}

export const useHlsPlayer = ({
  videoRef,
  streamUrl,
  initialPosition,
  isHlsStream,
  onError,
  onFragLoaded,
  qualityRecommendation,
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

  // Store recommendation in ref to avoid effect re-runs
  const qualityRecommendationRef = useRef(qualityRecommendation)
  qualityRecommendationRef.current = qualityRecommendation

  // Change quality level
  const changeQuality = useCallback((height: number, bandwidth?: number): number | null => {
    const hls = hlsRef.current
    if (!hls) {
      return null
    }

    if (height === 0) {
      // Auto quality - enable ABR
      hls.currentLevel = -1
      setCurrentQuality(0)
      setCurrentBandwidth(null)
      return -1
    }

    // Find matching level
    let levelIndex: number
    if (bandwidth) {
      levelIndex = hls.levels.findIndex(
        (level) => level.height === height && level.bitrate === bandwidth
      )
    } else {
      levelIndex = hls.levels.findIndex((level) => level.height === height)
    }

    if (levelIndex !== -1) {
      hls.currentLevel = levelIndex
      const selectedLevel = hls.levels[levelIndex]
      setCurrentQuality(height)
      setCurrentBandwidth(selectedLevel.bitrate || null)
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
    if (!video || !streamUrl) {
      return
    }

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
      xhrSetup: (xhr, _url) => {
        const authHeaders = getAuthHeaders()
        if (authHeaders['Authorization']) {
          xhr.setRequestHeader('Authorization', authHeaders['Authorization'])
        }
      },
    })
    hlsRef.current = hls

    // Load source and attach to video
    hls.loadSource(streamUrl)
    hls.attachMedia(video)

    // Handle manifest parsed - extract quality levels and audio tracks
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      const levels = hls.levels

      if (levels && levels.length > 0) {
        const qualities = levels
          .map((level, index) => {
            const url = level.url?.[0] || ''
            const isOriginal = url.includes('/original/') || level.name === 'original'
            return {
              height: level.height,
              bandwidth: level.bitrate,
              index,
              isOriginal,
            }
          })
          .filter((q) => q.height > 0)
          .sort((a, b) => {
            if (b.height !== a.height) {
              return b.height - a.height
            }
            return b.bandwidth - a.bandwidth
          })

        setAvailableQualities(qualities)

        // Apply recommended quality if available
        const rec = qualityRecommendationRef.current
        if (rec?.height && rec?.isReady) {
          let levelIndex = hls.levels.findIndex((level) => level.height === rec.height)

          if (levelIndex === -1) {
            const recHeight = rec.height
            const sortedLevels = hls.levels
              .map((level, index) => ({
                level,
                index,
                diff: Math.abs(level.height - recHeight),
                isHigher: level.height >= recHeight
              }))
              .filter((item) => item.level.height > 0)
              .sort((a, b) => {
                const is4KClass = recHeight >= 1440
                if (is4KClass) {
                  if (a.isHigher && !b.isHigher) {
                    return -1
                  }
                  if (!a.isHigher && b.isHigher) {
                    return 1
                  }
                }
                return a.diff - b.diff
              })

            if (sortedLevels.length > 0) {
              levelIndex = sortedLevels[0].index
            }
          }

          if (levelIndex !== -1) {
            hls.currentLevel = levelIndex
            setCurrentQuality(hls.levels[levelIndex].height)
          }
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

      // Extract subtitle tracks and enable native subtitle rendering
      const subtitleTracks = hls.subtitleTracks
      if (subtitleTracks && subtitleTracks.length > 0) {
        const tracks = subtitleTracks.map((track, index) => ({
          id: index,
          name: track.name || `Subtitle ${index + 1}`,
          language: track.lang || 'Unknown',
        }))
        setAvailableSubtitleTracks(tracks)
        // Enable subtitle display so HLS.js loads the VTT and adds to video.textTracks
        hls.subtitleDisplay = true
        // Set default subtitle track if one is marked default
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

    // Error handling
    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            onErrorRef.current('Network issue: Retrying...')
            hls.startLoad()
            break
          case Hls.ErrorTypes.MEDIA_ERROR:
            onErrorRef.current('Media error: Recovering...')
            hls.recoverMediaError()
            break
          default:
            logger.error('Fatal HLS error:', data.details)
            onErrorRef.current(`Playback error: ${data.details || 'Unknown error'}`)
            hls.destroy()
            hlsRef.current = null
            break
        }
      }
    })

    return () => {
      if (hls) {
        hls.destroy()
        hlsRef.current = null
      }
    }
  // Note: onError and onFragLoaded are stored in refs to avoid triggering re-initialization
  // eslint-disable-next-line react-hooks/exhaustive-deps
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
    setCurrentQuality,
    setCurrentBandwidth,
    setCurrentAudioTrack,
    changeQuality,
    changeAudioTrack,
    changeSubtitleTrack,
    loadSource,
  }
}
