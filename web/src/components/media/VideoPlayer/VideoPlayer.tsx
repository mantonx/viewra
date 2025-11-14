import { useEffect, useRef, useState } from 'react'
import Hls from 'hls.js'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { Button } from '@/components/ui'
import type { VideoPlayerProps } from './VideoPlayer.types'

export const VideoPlayer = ({ mediaId, streamUrl, initialPosition = 0, duration = 0, onClose }: VideoPlayerProps) => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const videoContainerRef = useRef<HTMLDivElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [videoDuration, setVideoDuration] = useState(duration)
  const [error, setError] = useState<string | null>(null)
  const [availableQualities, setAvailableQualities] = useState<Array<{ height: number; bandwidth: number }>>([])
  const [currentQuality, setCurrentQuality] = useState<number | null>(null)
  const progressUpdaterRef = useRef<ReturnType<typeof useProgressUpdater> | null>(null)

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

    // For direct streams, use native HTML5 video
    if (!isHlsStream) {
      video.src = streamUrl

      // Ensure video is unmuted for direct streams
      video.muted = false
      video.volume = 1.0

      if (initialPosition > 0) {
        video.currentTime = initialPosition
      }
      return
    }

    // Check for native HLS support (Safari/iOS)
    const canPlayHls = video.canPlayType('application/vnd.apple.mpegurl')

    if (canPlayHls) {
      // Native HLS support - just set the source
      video.src = streamUrl

      // Ensure video is unmuted
      video.muted = false
      video.volume = 1.0

      if (initialPosition > 0) {
        // For native HLS, set initial time after loadedmetadata
        const setInitialTime = () => {
          video.currentTime = initialPosition
          video.removeEventListener('loadedmetadata', setInitialTime)
        }
        video.addEventListener('loadedmetadata', setInitialTime)
      }
      return
    }

    // Use hls.js for browsers without native HLS support
    if (!Hls.isSupported()) {
      setError('Browser does not support HLS streaming')
      return
    }

    // Create hls.js instance
    const hls = new Hls({
      maxBufferLength: 30,
      maxMaxBufferLength: 60,
      enableWorker: true,
      lowLatencyMode: false,
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

      // Ensure video is unmuted when manifest is ready
      video.muted = false
      video.volume = 1.0

      // Set initial position after manifest is loaded
      if (initialPosition > 0) {
        video.currentTime = initialPosition
      }

      // Try to play with sound
      video.play().catch((err) => {
        console.warn('Autoplay with sound blocked:', err)
        // If autoplay with sound is blocked, the user will need to click play
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
      console.error('HLS.js error:', data)

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
  }, [streamUrl, initialPosition, isHlsStream])

  // Set up video event handlers for progress tracking
  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }

    // Handle video loaded metadata to get actual duration
    const handleLoadedMetadata = () => {
      if (video.duration && !isNaN(video.duration)) {
        setVideoDuration(video.duration)
      }

      // Ensure video is unmuted when metadata loads
      video.muted = false
      video.volume = 1.0
    }

    // Handle play event
    const handlePlay = () => {
      setIsPlaying(true)

      // Ensure video is unmuted when playback starts
      video.muted = false
      video.volume = 1.0

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
    const handleTimeUpdate = () => {
      if (progressUpdaterRef.current && isPlaying) {
        progressUpdaterRef.current.updateCurrentTime(video.currentTime)
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

    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)
    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('ended', handleEnded)

    return () => {
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('play', handlePlay)
      video.removeEventListener('pause', handlePause)
      video.removeEventListener('timeupdate', handleTimeUpdate)
      video.removeEventListener('ended', handleEnded)

      // Stop tracking when component unmounts
      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }
  }, [videoDuration, isPlaying])

  // Note: We don't need to recreate the progress updater when duration changes
  // The hook is already called at the top level with the initial duration
  // and it will handle updates through its own internal state

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

  const formatQualityLabel = (height: number): string => {
    if (height === 0) {
      return 'Auto'
    }
    if (height >= 2160) {
      return '4K'
    }
    if (height >= 1080) {
      return '1080p'
    }
    if (height >= 720) {
      return '720p'
    }
    if (height >= 480) {
      return '480p'
    }
    return `${height}p`
  }

  return (
    <div className="fixed inset-0 z-50 bg-black flex flex-col">
      {/* Header bar */}
      <div className="absolute top-0 left-0 right-0 z-10 bg-gradient-to-b from-black/80 to-transparent p-4">
        <div className="flex items-center justify-between">
          <h2 className="text-white text-lg font-semibold">Now Playing</h2>
          <div className="flex items-center gap-4">
            {/* Quality selector */}
            {availableQualities.length > 0 && (
              <div className="flex items-center gap-2">
                <span className="text-white text-sm">Quality:</span>
                <select
                  value={currentQuality || 0}
                  onChange={(e) => handleQualityChange(Number(e.target.value))}
                  className="bg-white/20 text-white text-sm rounded px-2 py-1 border border-white/30 hover:bg-white/30 focus:outline-none focus:ring-2 focus:ring-white/50"
                >
                  <option value={0}>Auto</option>
                  {availableQualities.map((quality) => (
                    <option key={quality.height} value={quality.height}>
                      {formatQualityLabel(quality.height)}
                    </option>
                  ))}
                </select>
              </div>
            )}
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
        </div>
      </div>

      {/* Error display */}
      {error && (
        <div className="absolute top-20 left-1/2 transform -translate-x-1/2 z-20 bg-red-600/90 text-white px-6 py-3 rounded-lg shadow-lg max-w-md">
          <p className="text-sm font-medium">{error}</p>
        </div>
      )}

      {/* Video player */}
      <div ref={videoContainerRef} className="flex-1 flex items-center justify-center">
        <video
          ref={videoRef}
          className="w-full h-full max-h-screen"
          controls
          autoPlay
          playsInline
        >
          Your browser does not support the video tag.
        </video>
      </div>

      {/* Footer info */}
      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-4 pointer-events-none">
        <div className="text-white text-sm">
          {initialPosition > 0 && (
            <p className="opacity-75">Resumed from {Math.floor(initialPosition / 60)}m {Math.floor(initialPosition % 60)}s</p>
          )}
          {currentQuality && currentQuality > 0 && (
            <p className="opacity-75 mt-1">Playing at {formatQualityLabel(currentQuality)}</p>
          )}
        </div>
      </div>
    </div>
  )
}

export type { VideoPlayerProps }
