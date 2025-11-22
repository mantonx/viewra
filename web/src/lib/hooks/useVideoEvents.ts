import { useEffect, useRef, useState } from 'react'
import { logger } from '../utils/logger'
import type { useProgressUpdater } from './useProgress'

type ProgressUpdater = ReturnType<typeof useProgressUpdater>

export interface UseVideoEventsReturn {
  isPlaying: boolean
  currentTime: number
  volume: number
  isMuted: boolean
  isFullscreen: boolean
  isPiP: boolean
  setVolume: (volume: number) => void
  setIsMuted: (muted: boolean) => void
  setIsFullscreen: (fullscreen: boolean) => void
}

/**
 * Hook for managing video element event listeners and state.
 * Handles play/pause, timeupdate, volume, fullscreen, and PiP events.
 */
export const useVideoEvents = (
  videoRef: React.RefObject<HTMLVideoElement>,
  videoDuration: number,
  progressUpdater: ProgressUpdater | null
): UseVideoEventsReturn => {
  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [volume, setVolume] = useState(1)
  const [isMuted, setIsMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [isPiP, setIsPiP] = useState(false)
  const lastTimeUpdateRef = useRef<number>(0)

  useEffect(() => {
    const video = videoRef.current
    if (!video) {return}

    // Play event
    const handlePlay = () => {
      setIsPlaying(true)
      logger.debug('Video playing')
    }

    // Pause event
    const handlePause = () => {
      setIsPlaying(false)
      logger.debug('Video paused')
    }

    // Time update (throttled to once per second)
    const handleTimeUpdate = () => {
      const currentSecond = Math.floor(video.currentTime)
      if (currentSecond !== lastTimeUpdateRef.current) {
        lastTimeUpdateRef.current = currentSecond
        setCurrentTime(currentSecond)

        // Update progress if we have a duration
        if (videoDuration > 0 && progressUpdater) {
          progressUpdater.updateCurrentTime(currentSecond)
        }
      }
    }

    // Volume change
    const handleVolumeChange = () => {
      setVolume(video.volume)
      setIsMuted(video.muted)
    }

    // Ended event
    const handleEnded = () => {
      logger.debug('Video ended')
      if (videoDuration > 0 && progressUpdater) {
        progressUpdater.immediateUpdate()
      }
    }

    // Loaded metadata
    const handleLoadedMetadata = () => {
      logger.debug('Video metadata loaded', {
        duration: video.duration,
        videoWidth: video.videoWidth,
        videoHeight: video.videoHeight,
      })
    }

    // Error
    const handleError = () => {
      const error = video.error
      if (error) {
        logger.error('Video error', {
          code: error.code,
          message: error.message,
        })
      }
    }

    // Attach event listeners
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)
    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('volumechange', handleVolumeChange)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('error', handleError)

    // Cleanup
    return () => {
      video.removeEventListener('play', handlePlay)
      video.removeEventListener('pause', handlePause)
      video.removeEventListener('timeupdate', handleTimeUpdate)
      video.removeEventListener('volumechange', handleVolumeChange)
      video.removeEventListener('ended', handleEnded)
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('error', handleError)
    }
  }, [videoRef, videoDuration, progressUpdater])

  // Fullscreen change event
  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement)
    }

    document.addEventListener('fullscreenchange', handleFullscreenChange)
    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange)
    }
  }, [])

  // Picture-in-Picture event
  useEffect(() => {
    const video = videoRef.current
    if (!video) {return}

    const handleEnterPiP = () => {
      setIsPiP(true)
      logger.debug('Entered Picture-in-Picture')
    }

    const handleLeavePiP = () => {
      setIsPiP(false)
      logger.debug('Left Picture-in-Picture')
    }

    video.addEventListener('enterpictureinpicture', handleEnterPiP)
    video.addEventListener('leavepictureinpicture', handleLeavePiP)

    return () => {
      video.removeEventListener('enterpictureinpicture', handleEnterPiP)
      video.removeEventListener('leavepictureinpicture', handleLeavePiP)
    }
  }, [videoRef])

  return {
    isPlaying,
    currentTime,
    volume,
    isMuted,
    isFullscreen,
    isPiP,
    setVolume,
    setIsMuted,
    setIsFullscreen,
  }
}
