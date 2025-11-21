import { Button } from '@/components/ui'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { logger } from '@/lib/utils/logger'
import Hls from 'hls.js'
import { useEffect, useRef, useState } from 'react'
import { VideoControls } from './VideoControls'
import type { VideoPlayerProps } from './VideoPlayer.types'

// HLS configuration constants
const HLS_CONFIG = {
  MAX_BUFFER_LENGTH: 30,
  MAX_MAX_BUFFER_LENGTH: 60,
  ENABLE_WORKER: true,
  LOW_LATENCY_MODE: false,
} as const

// Helper functions for video element manipulation
const ensureVideoUnmuted = (video: HTMLVideoElement) => {
  video.muted = false
  video.volume = 1.0
}

const setInitialPosition = (video: HTMLVideoElement, position: number, waitForMetadata = false) => {
  if (position <= 0) {
    return
  }

  if (waitForMetadata) {
    const setTime = () => {
      video.currentTime = position
      video.removeEventListener('loadedmetadata', setTime)
    }
    video.addEventListener('loadedmetadata', setTime)
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
  const [playbackSpeed, setPlaybackSpeed] = useState<number>(1)
  const [isBuffering, setIsBuffering] = useState<boolean>(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [volume, setVolume] = useState(1)
  const [isMuted, setIsMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const progressUpdaterRef = useRef<ReturnType<typeof useProgressUpdater> | null>(null)
  const lastTimeUpdateRef = useRef<number>(0)

  // Detect if this is an HLS stream or direct stream
  const isHlsStream = streamUrl.endsWith('.m3u8')

  // Initialize progress updater hook
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)
  progressUpdaterRef.current = progressUpdater

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
      setInitialPosition(video, initialPosition, true)
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

    // Create hls.js instance
    const hls = new Hls({
      maxBufferLength: HLS_CONFIG.MAX_BUFFER_LENGTH,
      maxMaxBufferLength: HLS_CONFIG.MAX_MAX_BUFFER_LENGTH,
      enableWorker: HLS_CONFIG.ENABLE_WORKER,
      lowLatencyMode: HLS_CONFIG.LOW_LATENCY_MODE,
    })
    hlsRef.current = hls

    // Load source and attach to video
    hls.loadSource(streamUrl)
    hls.attachMedia(video)

    // Handle manifest parsed - extract quality levels
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      const levels = hls.levels
      if (levels && levels.length > 0) {
        const qualities = levels
          .map((level, index) => ({
            height: level.height,
            bandwidth: level.bitrate,
            index,
          }))
          .filter((q) => q.height > 0)
          .sort((a, b) => b.height - a.height)

        // Remove duplicates by height
        const uniqueQualities = qualities.filter(
          (quality, index, self) => index === self.findIndex((q) => q.height === quality.height)
        )

        setAvailableQualities(uniqueQualities)
      }

      // Set initial position
      setInitialPosition(video, initialPosition)

      // Start muted to allow autoplay, will unmute on first play event
      video.muted = true
      video
        .play()
        .then(() => {
          // Successfully started - unmute immediately
          ensureVideoUnmuted(video)
          logger.debug('Video started with audio')
        })
        .catch((err) => {
          logger.warn('Autoplay blocked:', err)
          // If even muted autoplay is blocked, user will need to click play
        })
    })

    // Track current quality level changes
    hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
      const currentLevel = hls.levels[data.level]
      if (currentLevel && currentLevel.height) {
        setCurrentQuality(currentLevel.height)
      }
    })

    // Clear error on successful fragment load
    hls.on(Hls.Events.FRAG_LOADED, () => {
      if (error) {
        setError(null)
      }
    })

    // Error handling
    hls.on(Hls.Events.ERROR, (_event, data) => {
      logger.error('HLS.js error:', data)

      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            // Network error - usually means segments aren't ready yet
            // This can happen during progressive transcoding
            setError('Buffering issue: Waiting for video segments to be generated...')
            // Try to recover
            hls.startLoad()
            break
          case Hls.ErrorTypes.MEDIA_ERROR:
            // Media error - try to recover
            setError('Media error: Attempting to recover...')
            hls.recoverMediaError()
            break
          default:
            // Fatal error - cannot recover
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
  }, [streamUrl, initialPosition, isHlsStream, error])

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
      const currentSecond = Math.floor(video.currentTime)
      if (currentSecond !== lastTimeUpdateRef.current) {
        lastTimeUpdateRef.current = currentSecond
        setCurrentTime(video.currentTime)
        if (progressUpdaterRef.current && isPlaying) {
          progressUpdaterRef.current.updateCurrentTime(video.currentTime)
        }
      }
    }

    // Handle video ended - mark as complete (100%)
    const handleEnded = () => {
      setIsPlaying(false)
      if (progressUpdaterRef.current && videoDuration > 0) {
        progressUpdaterRef.current.stopTracking()
        // Mark as watched by updating to full duration
        progressUpdaterRef.current.updateCurrentTime(videoDuration)
        progressUpdaterRef.current.stopTracking()
      }
    }

    // Handle buffering events
    const handleWaiting = () => {
      setIsBuffering(true)
    }

    const handleCanPlay = () => {
      setIsBuffering(false)
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

    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)
    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('waiting', handleWaiting)
    video.addEventListener('canplay', handleCanPlay)
    video.addEventListener('volumechange', handleVolumeChange)
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
      document.removeEventListener('fullscreenchange', handleFullscreenChange)

      // Stop tracking when component unmounts
      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }
  }, [videoDuration, isPlaying, duration])

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

  // Handle quality selection
  const handleQualityChange = (height: number) => {
    const hls = hlsRef.current
    if (!hls) {
      return
    }

    if (height === 0) {
      // Auto quality - enable ABR
      hls.currentLevel = -1
      setCurrentQuality(0)
    } else {
      // Lock to specific quality
      const levelIndex = hls.levels.findIndex((level) => level.height === height)
      if (levelIndex !== -1) {
        hls.currentLevel = levelIndex
        setCurrentQuality(height)
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
    if (video) {
      video.currentTime = time
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
            className="text-white hover:bg-white/20"
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

        {/* Custom video controls */}
        <VideoControls
          videoRef={videoRef}
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={videoDuration}
          volume={volume}
          isMuted={isMuted}
          isFullscreen={isFullscreen}
          availableQualities={availableQualities}
          currentQuality={currentQuality}
          playbackSpeed={playbackSpeed}
          metadata={metadata}
          onPlayPause={handlePlayPause}
          onSeek={handleSeek}
          onVolumeChange={handleVolumeChangeControl}
          onMuteToggle={handleMuteToggle}
          onFullscreenToggle={handleFullscreenToggle}
          onQualityChange={handleQualityChange}
          onSpeedChange={handlePlaybackSpeedChange}
          onSkip={handleSkip}
        />
      </div>
    </div>
  )
}

export type { VideoPlayerProps, MediaMetadata } from './VideoPlayer.types'
