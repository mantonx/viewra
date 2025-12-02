/**
 * useVideoControls Hook
 * Provides control handlers for video playback, seeking, volume, etc.
 */

import { useCallback, useRef } from 'react'
import Hls from 'hls.js'
import { logger } from '@/lib/utils/logger'

// Threshold for triggering backend FFmpeg restart on seek
const LARGE_SEEK_THRESHOLD_SECONDS = 30

// Helper to ensure video is unmuted
const ensureVideoUnmuted = (video: HTMLVideoElement) => {
  video.muted = false
  video.volume = 1.0
}

export interface UseVideoControlsOptions {
  videoRef: React.RefObject<HTMLVideoElement | null>
  containerRef: React.RefObject<HTMLDivElement | null>
  hlsRef: React.RefObject<Hls | null>
  streamOffsetRef: React.RefObject<number>
  isSeekingRef: React.RefObject<boolean>
  isHlsStream: boolean
  videoDuration: number
  progressUpdater: {
    updateCurrentTime: (time: number) => void
    immediateUpdate: () => void
  } | null
  onTimeUpdate: (time: number) => void
}

export interface VideoControlHandlers {
  handlePlayPause: () => void
  handleSeek: (time: number) => void
  handleVolumeChange: (volume: number) => void
  handleMuteToggle: () => void
  handleFullscreenToggle: () => void
  handlePiPToggle: () => Promise<void>
  handleSkip: (seconds: number) => void
  handlePlaybackSpeedChange: (speed: number) => void
}

export const useVideoControls = ({
  videoRef,
  containerRef,
  hlsRef,
  streamOffsetRef,
  isSeekingRef,
  isHlsStream,
  videoDuration,
  progressUpdater,
  onTimeUpdate,
}: UseVideoControlsOptions): VideoControlHandlers => {
  const progressUpdaterRef = useRef(progressUpdater)
  progressUpdaterRef.current = progressUpdater
  const lastTimeUpdateRef = useRef<number>(0)
  const videoDurationRef = useRef(videoDuration)
  videoDurationRef.current = videoDuration
  const onTimeUpdateRef = useRef(onTimeUpdate)
  onTimeUpdateRef.current = onTimeUpdate

  const handlePlayPause = useCallback(() => {
    const video = videoRef.current
    if (video) {
      if (video.paused) {
        video.play()
      } else {
        video.pause()
      }
    }
  }, [videoRef])

  const handleSeek = useCallback((time: number) => {
    const video = videoRef.current
    const hls = hlsRef.current
    if (!video) {
      return
    }

    // Set seeking flag to prevent handleTimeUpdate from overriding our time
    if (isSeekingRef.current !== undefined) {
      (isSeekingRef as React.MutableRefObject<boolean>).current = true
    }

    // Save progress immediately
    if (progressUpdaterRef.current && videoDurationRef.current > 0) {
      progressUpdaterRef.current.updateCurrentTime(time)
      progressUpdaterRef.current.immediateUpdate()
    }

    // Update displayed time immediately
    onTimeUpdateRef.current(time)

    const streamOffset = streamOffsetRef.current || 0
    const actualTime = video.currentTime + streamOffset
    const seekDistance = Math.abs(time - actualTime)
    const isLargeSeek = seekDistance > LARGE_SEEK_THRESHOLD_SECONDS
    const potentialStreamTime = time - streamOffset

    const needsStreamRestart = isLargeSeek || potentialStreamTime < 0

    if (isHlsStream && hls?.url && needsStreamRestart) {
      const targetTime = time
      const baseUrl = hls.url.split('?')[0]
      const newUrl = `${baseUrl}?start=${time}`

      hls.loadSource(newUrl)

      const onFragBuffered = () => {
        (streamOffsetRef as React.MutableRefObject<number>).current = targetTime - video.currentTime
        lastTimeUpdateRef.current = Math.floor(video.currentTime)
        if (isSeekingRef.current !== undefined) {
          (isSeekingRef as React.MutableRefObject<boolean>).current = false
        }
        hls.off(Hls.Events.FRAG_BUFFERED, onFragBuffered)
      }
      hls.on(Hls.Events.FRAG_BUFFERED, onFragBuffered)

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
      const streamTime = time - streamOffset
      video.currentTime = streamTime

      setTimeout(() => {
        if (isSeekingRef.current !== undefined) {
          (isSeekingRef as React.MutableRefObject<boolean>).current = false
        }
      }, 100)
    }
  }, [videoRef, hlsRef, streamOffsetRef, isSeekingRef, isHlsStream])

  const handleVolumeChange = useCallback((newVolume: number) => {
    const video = videoRef.current
    if (video) {
      video.volume = newVolume
      if (newVolume > 0 && video.muted) {
        video.muted = false
      }
    }
  }, [videoRef])

  const handleMuteToggle = useCallback(() => {
    const video = videoRef.current
    if (video) {
      video.muted = !video.muted
    }
  }, [videoRef])

  const handleFullscreenToggle = useCallback(() => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen()
    } else {
      document.exitFullscreen()
    }
  }, [containerRef])

  const handlePiPToggle = useCallback(async () => {
    const video = videoRef.current
    if (!video) {
      return
    }

    try {
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture()
      } else if (document.pictureInPictureEnabled) {
        await video.requestPictureInPicture()
      }
    } catch (error) {
      logger.error('Picture-in-Picture error:', error)
    }
  }, [videoRef])

  const handleSkip = useCallback((seconds: number) => {
    const video = videoRef.current
    if (video) {
      video.currentTime = Math.max(0, Math.min(video.duration || videoDurationRef.current, video.currentTime + seconds))
    }
  }, [videoRef])

  const handlePlaybackSpeedChange = useCallback((speed: number) => {
    const video = videoRef.current
    if (video) {
      video.playbackRate = speed
    }
  }, [videoRef])

  return {
    handlePlayPause,
    handleSeek,
    handleVolumeChange,
    handleMuteToggle,
    handleFullscreenToggle,
    handlePiPToggle,
    handleSkip,
    handlePlaybackSpeedChange,
  }
}
