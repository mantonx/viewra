import { useEffect, useRef, useState } from 'react'
import shaka from 'shaka-player/dist/shaka-player.ui'
import 'shaka-player/dist/controls.css'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { Button } from '@/components/ui'
import type { VideoPlayerProps } from './VideoPlayer.types'

export const VideoPlayer = ({ mediaId, streamUrl, initialPosition = 0, duration = 0, onClose }: VideoPlayerProps) => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const videoContainerRef = useRef<HTMLDivElement>(null)
  const playerRef = useRef<shaka.Player | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [videoDuration, setVideoDuration] = useState(duration)
  const [error, setError] = useState<string | null>(null)
  const [availableQualities, setAvailableQualities] = useState<Array<{ height: number; bandwidth: number }>>([])
  const [currentQuality, setCurrentQuality] = useState<number | null>(null)
  const progressUpdaterRef = useRef<ReturnType<typeof useProgressUpdater> | null>(null)

  // Initialize progress updater hook
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)
  progressUpdaterRef.current = progressUpdater

  // Initialize Shaka Player
  useEffect(() => {
    const video = videoRef.current
    const container = videoContainerRef.current
    if (!video || !container) return

    // Check if browser supports DASH
    if (!shaka.Player.isBrowserSupported()) {
      setError('Browser does not support DASH streaming')
      return
    }

    // Create player instance
    const player = new shaka.Player()
    playerRef.current = player

    // Attach player to video element
    player.attach(video)

    // Configure player
    player.configure({
      streaming: {
        bufferingGoal: 30,
        rebufferingGoal: 2,
      },
    })

    // Set up error handling
    player.addEventListener('error', (event) => {
      const errorEvent = event as unknown as { detail: shaka.util.Error }
      console.error('Shaka Player error:', errorEvent.detail)
      setError(`Playback error: ${errorEvent.detail.message || 'Unknown error'}`)
    })

    // Track available qualities
    player.addEventListener('variantchanged', () => {
      const tracks = player.getVariantTracks()
      const qualities = tracks
        .filter((track) => track.height)
        .map((track) => ({
          height: track.height!,
          bandwidth: track.bandwidth,
        }))
        .sort((a, b) => b.height - a.height)

      // Remove duplicates by height
      const uniqueQualities = qualities.filter(
        (quality, index, self) => index === self.findIndex((q) => q.height === quality.height)
      )

      setAvailableQualities(uniqueQualities)

      // Get current quality
      const currentTrack = tracks.find((track) => track.active)
      if (currentTrack?.height) {
        setCurrentQuality(currentTrack.height)
      }
    })

    // Load DASH manifest
    player
      .load(streamUrl)
      .then(() => {
        // Set initial position if provided
        if (initialPosition > 0) {
          video.currentTime = initialPosition
        }
      })
      .catch((error) => {
        console.error('Failed to load DASH manifest:', error)
        setError(`Failed to load video: ${error.message || 'Unknown error'}`)
      })

    return () => {
      player.destroy()
      playerRef.current = null
    }
  }, [streamUrl, initialPosition])

  // Set up video event handlers for progress tracking
  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    // Handle video loaded metadata to get actual duration
    const handleLoadedMetadata = () => {
      if (video.duration && !isNaN(video.duration)) {
        setVideoDuration(video.duration)
      }
    }

    // Handle play event
    const handlePlay = () => {
      setIsPlaying(true)
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

  // Update progress updater when duration changes
  useEffect(() => {
    if (videoDuration > 0 && progressUpdaterRef.current) {
      // Recreate progress updater with new duration
      const newProgressUpdater = useProgressUpdater(mediaId, videoDuration)
      progressUpdaterRef.current = newProgressUpdater
    }
  }, [videoDuration, mediaId])

  // Handle quality selection
  const handleQualityChange = (height: number) => {
    const player = playerRef.current
    if (!player) return

    const tracks = player.getVariantTracks()
    const selectedTrack = tracks.find((track) => track.height === height)

    if (selectedTrack) {
      // Disable adaptive bitrate and lock to selected quality
      player.configure({
        abr: { enabled: height === 0 }, // Enable ABR only for "Auto"
      })

      if (height === 0) {
        // Auto quality - re-enable ABR
        player.configure({
          abr: { enabled: true },
        })
      } else {
        // Lock to specific quality
        player.selectVariantTrack(selectedTrack, true)
      }

      setCurrentQuality(height)
    }
  }

  const formatQualityLabel = (height: number): string => {
    if (height === 0) return 'Auto'
    if (height >= 2160) return '4K'
    if (height >= 1080) return '1080p'
    if (height >= 720) return '720p'
    if (height >= 480) return '480p'
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
