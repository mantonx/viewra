import { useEffect, useRef, useState } from 'react'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { Button } from '@/components/ui'
import type { VideoPlayerProps } from './VideoPlayer.types'

export const VideoPlayer = ({ mediaId, streamUrl, initialPosition = 0, duration = 0, onClose }: VideoPlayerProps) => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [videoDuration, setVideoDuration] = useState(duration)
  const progressUpdaterRef = useRef<ReturnType<typeof useProgressUpdater> | null>(null)

  // Initialize progress updater hook
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)
  progressUpdaterRef.current = progressUpdater

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    // Set initial position if provided
    if (initialPosition > 0) {
      video.currentTime = initialPosition
    }

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
  }, [initialPosition, videoDuration, isPlaying])

  // Update progress updater when duration changes
  useEffect(() => {
    if (videoDuration > 0 && progressUpdaterRef.current) {
      // Recreate progress updater with new duration
      const newProgressUpdater = useProgressUpdater(mediaId, videoDuration)
      progressUpdaterRef.current = newProgressUpdater
    }
  }, [videoDuration, mediaId])

  return (
    <div className="fixed inset-0 z-50 bg-black flex flex-col">
      {/* Header bar */}
      <div className="absolute top-0 left-0 right-0 z-10 bg-gradient-to-b from-black/80 to-transparent p-4">
        <div className="flex items-center justify-between">
          <h2 className="text-white text-lg font-semibold">Now Playing</h2>
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

      {/* Video player */}
      <div className="flex-1 flex items-center justify-center">
        <video
          ref={videoRef}
          className="w-full h-full max-h-screen"
          controls
          autoPlay
          src={streamUrl}
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
        </div>
      </div>
    </div>
  )
}

export type { VideoPlayerProps }
