import { Button } from '@/components/ui'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { useQualityRecommendation } from '@/lib/hooks/useQualityRecommendation'
import { useAutoQuality } from '@/lib/hooks/useAutoQuality'
import { usePlaybackAnalytics } from '@/lib/hooks/usePlaybackAnalytics'
import { useStreamStats } from '@/lib/hooks/useStreamStats'
import { logger } from '@/lib/utils/logger'
import { getAuthHeaders } from '@/lib/utils/authFetch'
import Hls from 'hls.js'
import { useCallback, useEffect, useRef, useState } from 'react'
import { VideoControls } from './VideoControls'
import { StatsPanel } from './StatsPanel'
import type { VideoPlayerProps } from './VideoPlayer.types'
import type { QualityRecommendationResponse } from '@/lib/api/adaptive'

// Threshold for triggering backend FFmpeg restart on seek (matches backend config)
const LARGE_SEEK_THRESHOLD_SECONDS = 30

// HLS configuration constants
// CRITICAL: Ultra-conservative buffer limits to prevent bufferFullError
// Segments are generated on-demand and served instantly from cache,
// so we need MINIMAL buffering to avoid overwhelming the browser's SourceBuffer
const HLS_CONFIG = {
  MAX_BUFFER_LENGTH: 30,        // Buffer 30 seconds ahead (5 segments)
  MAX_MAX_BUFFER_LENGTH: 60,    // Maximum buffer of 1 minute
  MAX_BUFFER_SIZE: 100 * 1000 * 1000,  // 100MB max buffer size
  MAX_BUFFER_HOLE: 2.0,         // Maximum gap tolerance (2s for keyframe alignment)
  ENABLE_WORKER: false,         // Disable worker - can cause audio issues
  LOW_LATENCY_MODE: false,
  DEBUG: false,                 // Disable debug logging for production
} as const

// Helper functions for video element manipulation
const ensureVideoUnmuted = (video: HTMLVideoElement) => {
  video.muted = false
  video.volume = 1.0
}

// HLS transcoded streams: Backend starts at ?start=X, but stream reports time=0.
// We track the offset to display correct position in scrubber without seeking.
const initializeStreamOffset = (
  streamOffsetRef: { current: number },
  initialPosition: number
) => {
  if (initialPosition > 0 && streamOffsetRef.current === 0) {
    streamOffsetRef.current = initialPosition
  }
}

const setInitialPosition = (video: HTMLVideoElement, position: number, waitForMetadata = false) => {
  if (position <= 0) {
    return
  }

  if (waitForMetadata) {
    // For HLS streams, we need to wait for enough data to be buffered before seeking
    const setTime = () => {
      if (video.seekable.length > 0) {
        const seekableEnd = video.seekable.end(video.seekable.length - 1)
        if (position <= seekableEnd) {
          video.currentTime = position
          video.removeEventListener('canplay', setTime)
        }
      }
    }
    video.addEventListener('canplay', setTime)
  } else {
    video.currentTime = position
  }
}

export const VideoPlayer = ({
  mediaId,
  streamUrl,
  initialPosition = 0,
  duration = 0,
  metadata,
  onClose,
  onTimeUpdate,
}: VideoPlayerProps) => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const videoContainerRef = useRef<HTMLDivElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [videoDuration, setVideoDuration] = useState(duration)
  const [error, setError] = useState<string | null>(null)
  const [availableQualities, setAvailableQualities] = useState<
    Array<{ height: number; bandwidth: number }>
  >([])
  const [currentQuality, setCurrentQuality] = useState<number | null>(null)
  const [currentBandwidth, setCurrentBandwidth] = useState<number | null>(null)
  const [playbackSpeed, setPlaybackSpeed] = useState<number>(1)
  const [isBuffering, setIsBuffering] = useState<boolean>(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [volume, setVolume] = useState(1)
  const [isMuted, setIsMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [isPiP, setIsPiP] = useState(false)
  const [availableAudioTracks, setAvailableAudioTracks] = useState<
    Array<{ id: number; name: string; language: string }>
  >([])
  const [currentAudioTrack, setCurrentAudioTrack] = useState<number>(-1)
  const [showDebugOverlay, setShowDebugOverlay] = useState(false)
  const progressUpdaterRef = useRef<ReturnType<typeof useProgressUpdater> | null>(null)
  const lastTimeUpdateRef = useRef<number>(0)

  // Flag to prevent handleTimeUpdate from overriding time during a seek operation
  const isSeekingRef = useRef<boolean>(false)

  // Track stream offset for progressive transcoding with seeking
  // When we seek to position X, FFmpeg restarts at that position, but the new stream starts at 0
  // We need to add this offset to display the correct time to the user
  const streamOffsetRef = useRef<number>(0)

  // Detect if this is an HLS stream or direct stream
  // Check for .m3u8 in the URL path (before query parameters)
  const isHlsStream = streamUrl.includes('.m3u8')

  // Initialize progress updater hook
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)
  progressUpdaterRef.current = progressUpdater

  // Get quality recommendation based on client capabilities
  const qualityRecommendation = useQualityRecommendation()
  const qualityRecommendationRef = useRef(qualityRecommendation)
  qualityRecommendationRef.current = qualityRecommendation // Keep ref in sync
  const [recommendedQuality, setRecommendedQuality] = useState<QualityRecommendationResponse | null>(null)

  // Auto quality control - monitors network and adapts quality
  const handleAutoQualityChange = useCallback((level: { name: string; height: number }, _reason: string) => {
    setCurrentQuality(level.height)
  }, [])

  const {
    isAutoMode,
    setAutoMode,
    recordSample,
    recordStall,
    networkStats,
  } = useAutoQuality({
    enabled: isHlsStream,
    hlsInstance: hlsRef.current,
    onQualityChange: handleAutoQualityChange,
  })

  // Stream stats for "Stats for Nerds" panel
  const { stats: streamStats, isLoading: streamStatsLoading } = useStreamStats({
    mediaId,
    videoRef,
    hlsRef,
    networkStats,
    isPlaying,
    playbackMode: isHlsStream ? 'transcode' : 'direct',
    streamOffset: streamOffsetRef.current,
    enabled: showDebugOverlay,
  })

  // Handle toggling auto mode from UI
  const handleAutoToggle = useCallback(() => {
    const hls = hlsRef.current
    if (!hls) {
      return
    }

    if (isAutoMode) {
      // Currently auto - stay on current level but disable auto switching
      setAutoMode(false)
    } else {
      // Enable auto mode - let HLS.js handle quality selection
      hls.currentLevel = -1
      setAutoMode(true)
      setCurrentQuality(null)
    }
  }, [isAutoMode, setAutoMode])

  // Playback analytics - tracks quality switches and playback metrics
  const {
    startSession,
    endSession,
    recordQualitySwitch,
    recordStall: recordAnalyticsStall,
  } = usePlaybackAnalytics({ enabled: true })

  // Initialize HLS player for HLS streams
  useEffect(() => {
    const video = videoRef.current
    const container = videoContainerRef.current
    if (!video || !container) {
      return
    }

    // If streamUrl is empty, show buffering indicator and wait
    if (!streamUrl) {
      setIsBuffering(true)
      return
    }

    // Clear buffering state when we have a URL
    setIsBuffering(false)

    // For direct streams, use native HTML5 video
    if (!isHlsStream) {
      video.src = streamUrl
      setInitialPosition(video, initialPosition)
      // Start muted to allow autoplay, unmute immediately after play starts
      video.muted = true
      video
        .play()
        .then(() => {
          ensureVideoUnmuted(video)
        })
        .catch(() => {
          // Autoplay blocked, user will need to click play
        })
      return
    }

    // Check for native HLS support (Safari/iOS)
    const canPlayHls = video.canPlayType('application/vnd.apple.mpegurl')

    if (canPlayHls) {
      // Native HLS support - just set the source
      video.src = streamUrl
      initializeStreamOffset(streamOffsetRef, initialPosition)
      // Start muted to allow autoplay, unmute after play starts
      video.muted = true
      video
        .play()
        .then(() => {
          ensureVideoUnmuted(video)
        })
        .catch(() => {
          // Autoplay blocked
        })
      return
    }

    // Use hls.js for browsers without native HLS support
    if (!Hls.isSupported()) {
      setError('Browser does not support HLS streaming')
      return
    }

    // Create hls.js instance with minimal buffering to prevent bufferFullError
    const hls = new Hls({
      debug: HLS_CONFIG.DEBUG,
      enableWorker: HLS_CONFIG.ENABLE_WORKER,
      lowLatencyMode: HLS_CONFIG.LOW_LATENCY_MODE,
      // Buffer management - keep it moderate to avoid bufferFullError
      maxBufferLength: HLS_CONFIG.MAX_BUFFER_LENGTH,
      maxMaxBufferLength: HLS_CONFIG.MAX_MAX_BUFFER_LENGTH,
      maxBufferSize: HLS_CONFIG.MAX_BUFFER_SIZE,
      maxBufferHole: HLS_CONFIG.MAX_BUFFER_HOLE,
      // Back buffer management
      backBufferLength: 90,  // Keep 90 seconds behind for seeking
      // Prevent stalls
      highBufferWatchdogPeriod: 1,
      // Fragment retry
      fragLoadingMaxRetry: 6,
      fragLoadingMaxRetryTimeout: 64000,
      // Nudge forward on stalls
      nudgeOffset: 0.1,
      nudgeMaxRetry: 10,
      // Add auth headers to all HLS.js requests (manifests and segments)
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
            // Check if this is the "original" quality by looking at the URL or name
            // The master playlist sets NAME="original" for the source quality variant
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
          // Sort by height descending, then by bandwidth descending for same height
          .sort((a, b) => {
            if (b.height !== a.height) {
              return b.height - a.height
            }
            return b.bandwidth - a.bandwidth
          })

        // Keep all bitrate variants - QualitySelector groups by resolution and shows bitrate options
        setAvailableQualities(qualities)

        // Apply recommended quality if available (use ref to avoid re-running effect on recommendation changes)
        const qr = qualityRecommendationRef.current
        if (qr.recommendation && qr.isReady) {
          const rec = qr.recommendation
          setRecommendedQuality(rec)

          // Find matching quality level by height (exact match or closest available)
          if (rec.height) {
            let levelIndex = hls.levels.findIndex((level) => level.height === rec.height)

            // If no exact match, find closest quality (prefer lower to avoid buffering)
            if (levelIndex === -1) {
              // Filter out auto quality (height 0) and find closest real quality
              const recHeight = rec.height // TypeScript narrowing
              const sortedLevels = hls.levels
                .map((level, index) => ({
                  level,
                  index,
                  diff: Math.abs(level.height - recHeight),
                  isHigher: level.height >= recHeight
                }))
                .filter((item) => item.level.height > 0) // Exclude auto quality
                .sort((a, b) => {
                  // For high-quality recommendations (4K class: 1440p+), prefer higher over lower
                  // because the recommendation already accounts for capabilities
                  // For lower recommendations, prefer lower to avoid buffering
                  const is4KClass = recHeight >= 1440

                  if (is4KClass) {
                    // For 4K recommendations, prefer higher quality if close enough
                    // (e.g., prefer 2160p over 1080p for a 1606p recommendation)
                    if (a.isHigher && !b.isHigher) { return -1 }
                    if (!a.isHigher && b.isHigher) { return 1 }
                  }

                  // Otherwise sort by difference - closest quality wins
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
      }

      // Extract audio tracks if available
      const audioTracks = hls.audioTracks
      if (audioTracks && audioTracks.length > 0) {
        const tracks = audioTracks.map((track, index) => ({
          id: index,
          name: track.name || `Track ${index + 1}`,
          language: track.lang || 'Unknown',
        }))
        setAvailableAudioTracks(tracks)

        // Set initial audio track (HLS.js defaults to track 0)
        setCurrentAudioTrack(hls.audioTrack)
      }

      initializeStreamOffset(streamOffsetRef, initialPosition)

      // Start muted to allow autoplay, will unmute on first play event
      video.muted = true
      video
        .play()
        .then(() => {
          // Successfully started - unmute immediately
          ensureVideoUnmuted(video)
        })
        .catch(() => {
          // Autoplay blocked - user will need to click play
        })
    })

    // Track current quality level changes
    hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
      const currentLevel = hls.levels[data.level]
      if (currentLevel && currentLevel.height) {
        setCurrentQuality(currentLevel.height)
        setCurrentBandwidth(currentLevel.bitrate || null)
      }
    })

    // Track audio track changes
    hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_event, data) => {
      setCurrentAudioTrack(data.id)
    })

    // Clear error on successful fragment load and record network sample
    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      if (error) {
        setError(null)
      }
      // Record network sample for adaptive quality monitoring
      if (data.frag?.stats) {
        const stats = data.frag.stats
        const bytes = stats.total || 0
        const durationMs = stats.loading?.end && stats.loading?.start
          ? stats.loading.end - stats.loading.start
          : 0
        if (bytes > 0 && durationMs > 0) {
          recordSample(bytes, durationMs)
        }
      }
    })

    // Error handling
    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            // Network error - usually means segments aren't ready yet
            setError('Network issue: Retrying...')
            // Try to recover
            hls.startLoad()
            break
          case Hls.ErrorTypes.MEDIA_ERROR:
            // Media error - try to recover
            setError('Media error: Recovering...')
            hls.recoverMediaError()
            break
          default:
            // Fatal error - cannot recover
            logger.error('Fatal HLS error:', data.details)
            setError(`Playback error: ${data.details || 'Unknown error'}`)
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
  }, [streamUrl, initialPosition, isHlsStream, error, recordSample])

  // Set up video event handlers for progress tracking
  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }

    // Handle video loaded metadata to get actual duration
    const handleLoadedMetadata = () => {
      // For HLS streams being progressively transcoded, the video duration starts small
      // and increments as more segments are created. Prefer the database duration if available.
      // Only use video.duration if we don't have a duration prop or if it's larger (more accurate)
      if (video.duration && !isNaN(video.duration)) {
        if (duration && duration > 0) {
          // We have a duration from the database - use it instead of the HLS reported duration
          setVideoDuration(duration)
        } else {
          // No database duration - use what the video reports
          setVideoDuration(video.duration)
        }
      }

      ensureVideoUnmuted(video)
    }

    // Handle play event
    const handlePlay = () => {
      setIsPlaying(true)
      ensureVideoUnmuted(video)

      // Start analytics session on first play
      startSession(mediaId)

      if (progressUpdaterRef.current && videoDuration > 0) {
        progressUpdaterRef.current.startTracking(video.currentTime)
      }
    }

    // Handle pause event - update progress
    const handlePause = () => {
      setIsPlaying(false)
      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }

    // Handle time update - track current time for periodic updates
    // Throttle to once per second to reduce re-renders (timeupdate fires 4-15x per second)
    const handleTimeUpdate = () => {
      // Skip time updates while seeking to prevent race conditions
      if (isSeekingRef.current) {
        return
      }

      const currentSecond = Math.floor(video.currentTime)
      if (currentSecond !== lastTimeUpdateRef.current) {
        lastTimeUpdateRef.current = currentSecond
        // Add stream offset to show actual position in the video
        const actualTime = video.currentTime + streamOffsetRef.current
        setCurrentTime(actualTime)
        if (progressUpdaterRef.current && isPlaying) {
          progressUpdaterRef.current.updateCurrentTime(actualTime)
        }
        // Notify parent component of time update (for URL sync)
        if (onTimeUpdate) {
          onTimeUpdate(actualTime)
        }
      }
    }

    // Handle video ended - mark as complete (100%)
    const handleEnded = () => {
      setIsPlaying(false)
      // End analytics session
      endSession()

      if (progressUpdaterRef.current && videoDuration > 0) {
        progressUpdaterRef.current.stopTracking()
        // Mark as watched by updating to full duration
        progressUpdaterRef.current.updateCurrentTime(videoDuration)
        progressUpdaterRef.current.stopTracking()
      }
    }

    // Handle buffering events
    const stallStartRef = { current: 0 }
    const handleWaiting = () => {
      setIsBuffering(true)
      stallStartRef.current = Date.now()
      // Record stall for network monitoring
      recordStall()
    }

    const handleCanPlay = () => {
      // Record stall duration for analytics if we were buffering
      if (stallStartRef.current > 0) {
        const stallDuration = Date.now() - stallStartRef.current
        if (stallDuration > 100) { // Only record stalls > 100ms
          recordAnalyticsStall(stallDuration)
        }
        stallStartRef.current = 0
      }
      setIsBuffering(false)
      // Ensure video is unmuted when ready to play
      ensureVideoUnmuted(video)
    }

    // Handle volume changes
    const handleVolumeChange = () => {
      setVolume(video.volume)
      setIsMuted(video.muted)
    }

    // Handle fullscreen changes
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement)
    }

    // Handle PiP changes
    const handlePiPEnter = () => {
      setIsPiP(true)
    }

    const handlePiPExit = () => {
      setIsPiP(false)
    }

    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)
    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('waiting', handleWaiting)
    video.addEventListener('canplay', handleCanPlay)
    video.addEventListener('volumechange', handleVolumeChange)
    video.addEventListener('enterpictureinpicture', handlePiPEnter)
    video.addEventListener('leavepictureinpicture', handlePiPExit)
    document.addEventListener('fullscreenchange', handleFullscreenChange)

    return () => {
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('play', handlePlay)
      video.removeEventListener('pause', handlePause)
      video.removeEventListener('timeupdate', handleTimeUpdate)
      video.removeEventListener('ended', handleEnded)
      video.removeEventListener('waiting', handleWaiting)
      video.removeEventListener('canplay', handleCanPlay)
      video.removeEventListener('volumechange', handleVolumeChange)
      video.removeEventListener('enterpictureinpicture', handlePiPEnter)
      video.removeEventListener('leavepictureinpicture', handlePiPExit)
      document.removeEventListener('fullscreenchange', handleFullscreenChange)

      // Stop tracking when component unmounts
      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }
  }, [videoDuration, isPlaying, duration, mediaId, onTimeUpdate, startSession, endSession, recordStall, recordAnalyticsStall])

  // Note: We don't need to recreate the progress updater when duration changes
  // The hook is already called at the top level with the initial duration
  // and it will handle updates through its own internal state

  // Keyboard shortcuts for video control
  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input field
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      switch (e.key) {
        case ' ':
        case 'k': // Play/pause
          e.preventDefault()
          if (video.paused) {
            video.play()
          } else {
            video.pause()
          }
          break
        case 'ArrowLeft':
        case 'j': // Rewind 10 seconds
          e.preventDefault()
          video.currentTime = Math.max(0, video.currentTime - 10)
          break
        case 'ArrowRight':
        case 'l': // Forward 10 seconds
          e.preventDefault()
          video.currentTime = Math.min(video.duration || videoDuration, video.currentTime + 10)
          break
        case 'ArrowUp': // Volume up
          e.preventDefault()
          video.volume = Math.min(1, video.volume + 0.1)
          break
        case 'ArrowDown': // Volume down
          e.preventDefault()
          video.volume = Math.max(0, video.volume - 0.1)
          break
        case 'm': // Mute/unmute
          e.preventDefault()
          video.muted = !video.muted
          break
        case 'f': // Fullscreen toggle
          e.preventDefault()
          if (!document.fullscreenElement) {
            videoContainerRef.current?.requestFullscreen()
          } else {
            document.exitFullscreen()
          }
          break
        case '0':
        case 'Home': // Jump to start
          e.preventDefault()
          video.currentTime = 0
          break
        case 'End': // Jump to end
          e.preventDefault()
          video.currentTime = video.duration || videoDuration
          break
        case 'd':
        case 'D': // Toggle debug overlay
          e.preventDefault()
          setShowDebugOverlay(prev => !prev)
          break
      }

      // Number keys 1-9 for seeking to percentage
      if (e.key >= '1' && e.key <= '9') {
        e.preventDefault()
        const percentage = parseInt(e.key) / 10
        video.currentTime = (video.duration || videoDuration) * percentage
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [videoDuration])

  // Browser close progress save - captures progress when user closes tab/window
  useEffect(() => {
    const handleBeforeUnload = () => {
      // Only save if we have valid progress data
      if (currentTime > 0 && videoDuration > 0) {
        // Use sendBeacon for guaranteed delivery during page unload
        // This API is specifically designed to work even as the page is closing
        const data = JSON.stringify({
          media_id: mediaId,
          user_id: 1, // Default user
          progress_seconds: currentTime,
          duration_seconds: videoDuration,
        })

        // Construct the full API URL
        const apiUrl = `${window.location.origin}/api/progress`

        // sendBeacon sends a POST request that completes even if the page unloads
        // Returns true if the browser accepted the request for delivery
        navigator.sendBeacon(apiUrl, new Blob([data], { type: 'application/json' }))
      }
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [mediaId, currentTime, videoDuration])

  // Handle quality selection
  const handleQualityChange = (height: number, bandwidth?: number) => {
    const hls = hlsRef.current
    const video = videoRef.current
    if (!hls) {
      return
    }

    const previousQuality = currentQuality ? `${currentQuality}p` : null

    if (height === 0) {
      // Auto quality - enable ABR
      hls.currentLevel = -1
      setCurrentQuality(0)
      setCurrentBandwidth(null)
      setAutoMode(true)
      // Record manual switch to auto
      recordQualitySwitch(
        previousQuality,
        'auto',
        'user_manual',
        video?.currentTime || 0,
        null,
        null
      )
    } else {
      // Lock to specific quality - find by height and optionally bandwidth
      let levelIndex: number
      if (bandwidth) {
        // Find exact match by height AND bandwidth
        levelIndex = hls.levels.findIndex(
          (level) => level.height === height && level.bitrate === bandwidth
        )
      } else {
        // Fall back to just height match
        levelIndex = hls.levels.findIndex((level) => level.height === height)
      }

      if (levelIndex !== -1) {
        hls.currentLevel = levelIndex
        const selectedLevel = hls.levels[levelIndex]
        setCurrentQuality(height)
        setCurrentBandwidth(selectedLevel.bitrate || null)
        setAutoMode(false)
        // Record manual quality switch
        recordQualitySwitch(
          previousQuality,
          `${height}p`,
          'user_manual',
          video?.currentTime || 0,
          null,
          null
        )
      }
    }
  }

  // Handle playback speed change
  const handlePlaybackSpeedChange = (speed: number) => {
    const video = videoRef.current
    if (video) {
      video.playbackRate = speed
      setPlaybackSpeed(speed)
    }
  }

  // Handle audio track change
  const handleAudioTrackChange = (trackId: number) => {
    const hls = hlsRef.current
    if (!hls) {
      return
    }

    // Set the audio track using HLS.js API
    hls.audioTrack = trackId
    setCurrentAudioTrack(trackId)
  }

  // Control handlers for VideoControls component
  const handlePlayPause = () => {
    const video = videoRef.current
    if (video) {
      if (video.paused) {
        video.play()
      } else {
        video.pause()
      }
    }
  }

  const handleSeek = (time: number) => {
    const video = videoRef.current
    const hls = hlsRef.current
    if (!video) {
      return
    }

    // Set seeking flag to prevent handleTimeUpdate from overriding our time
    isSeekingRef.current = true

    // Save progress immediately to prevent loss if user closes video after seeking
    if (progressUpdaterRef.current && videoDuration > 0) {
      progressUpdaterRef.current.updateCurrentTime(time)
      progressUpdaterRef.current.immediateUpdate()
    }

    // Update displayed time immediately
    setCurrentTime(time)

    const actualTime = video.currentTime + streamOffsetRef.current
    const seekDistance = Math.abs(time - actualTime)
    const isLargeSeek = seekDistance > LARGE_SEEK_THRESHOLD_SECONDS
    const potentialStreamTime = time - streamOffsetRef.current

    // We need a stream restart if:
    // 1. The seek distance is large (>30 seconds), OR
    // 2. The target time is before the current stream's start point
    const needsStreamRestart = isLargeSeek || potentialStreamTime < 0

    if (isHlsStream && hls?.url && needsStreamRestart) {
      // Large seek: Reload manifest to restart FFmpeg at new position
      const targetTime = time
      const baseUrl = hls.url.split('?')[0]
      const newUrl = `${baseUrl}?start=${time}`

      hls.loadSource(newUrl)

      // Wait for first fragment to buffer before calculating offset.
      // The backend may serve cached segments, so video.currentTime won't necessarily be 0.
      // We calculate: offset = targetTime - video.currentTime
      // So that: displayedTime = video.currentTime + offset = targetTime
      const onFragBuffered = () => {
        streamOffsetRef.current = targetTime - video.currentTime
        lastTimeUpdateRef.current = Math.floor(video.currentTime)
        isSeekingRef.current = false
        hls.off(Hls.Events.FRAG_BUFFERED, onFragBuffered)
      }
      hls.on(Hls.Events.FRAG_BUFFERED, onFragBuffered)

      // Start playback after manifest loads
      const onManifestParsed = () => {
        if (!video.paused) {
          video.play().then(() => ensureVideoUnmuted(video)).catch(() => {})
        } else {
          ensureVideoUnmuted(video)
        }
        hls.off(Hls.Events.MANIFEST_PARSED, onManifestParsed)
      }
      hls.on(Hls.Events.MANIFEST_PARSED, onManifestParsed)
    } else {
      // Small seek within current stream
      const streamTime = time - streamOffsetRef.current
      video.currentTime = streamTime

      // Clear seeking flag after video has had time to update
      setTimeout(() => {
        isSeekingRef.current = false
      }, 100)
    }
  }

  const handleVolumeChangeControl = (newVolume: number) => {
    const video = videoRef.current
    if (video) {
      video.volume = newVolume
      if (newVolume > 0 && video.muted) {
        video.muted = false
      }
    }
  }

  const handleMuteToggle = () => {
    const video = videoRef.current
    if (video) {
      video.muted = !video.muted
    }
  }

  const handleFullscreenToggle = () => {
    if (!document.fullscreenElement) {
      videoContainerRef.current?.requestFullscreen()
    } else {
      document.exitFullscreen()
    }
  }

  const handlePiPToggle = async () => {
    const video = videoRef.current
    if (!video) {
      return
    }

    try {
      if (document.pictureInPictureElement) {
        // Already in PiP mode, exit it
        await document.exitPictureInPicture()
      } else if (document.pictureInPictureEnabled) {
        // Enter PiP mode
        await video.requestPictureInPicture()
      }
    } catch (error) {
      logger.error('Picture-in-Picture error:', error)
    }
  }

  const handleSkip = (seconds: number) => {
    const video = videoRef.current
    if (video) {
      video.currentTime = Math.max(0, Math.min(video.duration || videoDuration, video.currentTime + seconds))
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black flex flex-col">
      {/* Close button in top-right corner */}
      <div className="absolute top-4 right-4 z-30">
        {onClose && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="text-white hover:bg-white/20 cursor-pointer"
          >
            Close
          </Button>
        )}
      </div>

      {/* Error display */}
      {error && (
        <div className="absolute top-20 left-1/2 transform -translate-x-1/2 z-20 bg-red-600/90 text-white px-6 py-3 rounded-lg shadow-lg max-w-md">
          <p className="text-sm font-medium">{error}</p>
        </div>
      )}

      {/* Buffering indicator */}
      {isBuffering && (
        <div className="absolute inset-0 z-20 flex items-center justify-center pointer-events-none">
          {/* Clean spinning arc with gradient fade - creates smooth loading effect */}
          <div
            className="w-16 h-16 rounded-full animate-spin"
            style={{
              border: '4px solid transparent',
              borderTopColor: 'white',
              borderRightColor: 'rgba(255, 255, 255, 0.3)',
              borderBottomColor: 'rgba(255, 255, 255, 0.1)',
            }}
          />
        </div>
      )}

      {/* Video player */}
      <div
        ref={videoContainerRef}
        className="flex-1 flex items-center justify-center bg-black relative"
      >
        <video
          ref={videoRef}
          className="w-full h-full max-h-screen cursor-pointer"
          style={{ objectFit: 'contain' }}
          autoPlay
          playsInline
          onClick={handlePlayPause}
        >
          Your browser does not support the video tag.
        </video>

        {/* Stats for Nerds panel - toggle with 'D' key */}
        <StatsPanel
          stats={streamStats}
          networkStats={networkStats}
          isVisible={showDebugOverlay}
          onClose={() => setShowDebugOverlay(false)}
          isLoading={streamStatsLoading}
        />


        {/* Custom video controls */}
        <VideoControls
          videoRef={videoRef}
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={videoDuration}
          volume={volume}
          isMuted={isMuted}
          isFullscreen={isFullscreen}
          isPiP={isPiP}
          availableQualities={availableQualities}
          currentQuality={currentQuality}
          currentBandwidth={currentBandwidth}
          recommendedQuality={recommendedQuality}
          autoMode={isAutoMode}
          availableAudioTracks={availableAudioTracks}
          currentAudioTrack={currentAudioTrack}
          playbackSpeed={playbackSpeed}
          metadata={metadata}
          showStats={showDebugOverlay}
          onPlayPause={handlePlayPause}
          onSeek={handleSeek}
          onVolumeChange={handleVolumeChangeControl}
          onMuteToggle={handleMuteToggle}
          onFullscreenToggle={handleFullscreenToggle}
          onPiPToggle={handlePiPToggle}
          onQualityChange={handleQualityChange}
          onAutoToggle={handleAutoToggle}
          onAudioTrackChange={handleAudioTrackChange}
          onSpeedChange={handlePlaybackSpeedChange}
          onSkip={handleSkip}
          onToggleStats={() => setShowDebugOverlay(prev => !prev)}
        />
      </div>
    </div>
  )
}

export type { VideoPlayerProps, MediaMetadata } from './VideoPlayer.types'
