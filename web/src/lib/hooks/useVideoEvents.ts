/**
 * useVideoEvents Hook
 * Handles video element event listeners for playback state, buffering,
 * progress tracking, and analytics.
 */

import { useEffect, useRef } from 'react'
import { ensureVideoUnmuted } from '@/lib/utils/videoUtils'

export interface UseVideoEventsOptions {
  videoRef: React.RefObject<HTMLVideoElement | null>
  mediaId: number
  duration: number
  videoDuration: number
  isPlaying: boolean
  streamOffsetRef: React.RefObject<number>
  isSeekingRef: React.RefObject<boolean>
  onPlay: () => void
  onPause: () => void
  onTimeUpdate: (time: number) => void
  onEnded: () => void
  onBufferingStart: () => void
  onBufferingEnd: (stallDuration: number) => void
  onVolumeChange: (volume: number, muted: boolean) => void
  onFullscreenChange: (isFullscreen: boolean) => void
  onPiPEnter: () => void
  onPiPExit: () => void
  onDurationChange: (duration: number) => void
  startAnalyticsSession: (mediaId: number) => void
  recordStall: () => void
  progressUpdater: {
    startTracking: (time: number) => void
    stopTracking: () => void
    updateCurrentTime: (time: number) => void
  } | null
}

export const useVideoEvents = ({
  videoRef,
  mediaId,
  duration,
  videoDuration,
  isPlaying,
  streamOffsetRef,
  isSeekingRef,
  onPlay,
  onPause,
  onTimeUpdate,
  onEnded,
  onBufferingStart,
  onBufferingEnd,
  onVolumeChange,
  onFullscreenChange,
  onPiPEnter,
  onPiPExit,
  onDurationChange,
  startAnalyticsSession,
  recordStall,
  progressUpdater,
}: UseVideoEventsOptions) => {
  const lastTimeUpdateRef = useRef<number>(0)
  const stallStartRef = useRef<number>(0)

  // Store all callbacks and changing values in refs to avoid effect re-runs
  const progressUpdaterRef = useRef(progressUpdater)
  progressUpdaterRef.current = progressUpdater
  const videoDurationRef = useRef(videoDuration)
  videoDurationRef.current = videoDuration
  const isPlayingRef = useRef(isPlaying)
  isPlayingRef.current = isPlaying
  const durationRef = useRef(duration)
  durationRef.current = duration

  // Callback refs
  const onPlayRef = useRef(onPlay)
  onPlayRef.current = onPlay
  const onPauseRef = useRef(onPause)
  onPauseRef.current = onPause
  const onTimeUpdateRef = useRef(onTimeUpdate)
  onTimeUpdateRef.current = onTimeUpdate
  const onEndedRef = useRef(onEnded)
  onEndedRef.current = onEnded
  const onBufferingStartRef = useRef(onBufferingStart)
  onBufferingStartRef.current = onBufferingStart
  const onBufferingEndRef = useRef(onBufferingEnd)
  onBufferingEndRef.current = onBufferingEnd
  const onVolumeChangeRef = useRef(onVolumeChange)
  onVolumeChangeRef.current = onVolumeChange
  const onFullscreenChangeRef = useRef(onFullscreenChange)
  onFullscreenChangeRef.current = onFullscreenChange
  const onPiPEnterRef = useRef(onPiPEnter)
  onPiPEnterRef.current = onPiPEnter
  const onPiPExitRef = useRef(onPiPExit)
  onPiPExitRef.current = onPiPExit
  const onDurationChangeRef = useRef(onDurationChange)
  onDurationChangeRef.current = onDurationChange
  const startAnalyticsSessionRef = useRef(startAnalyticsSession)
  startAnalyticsSessionRef.current = startAnalyticsSession
  const recordStallRef = useRef(recordStall)
  recordStallRef.current = recordStall

  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }

    const handleLoadedMetadata = () => {
      if (video.duration && !isNaN(video.duration)) {
        if (durationRef.current && durationRef.current > 0) {
          onDurationChangeRef.current(durationRef.current)
        } else {
          onDurationChangeRef.current(video.duration)
        }
      }
      ensureVideoUnmuted(video)
    }

    const handlePlay = () => {
      onPlayRef.current()
      ensureVideoUnmuted(video)
      startAnalyticsSessionRef.current(mediaId)

      if (progressUpdaterRef.current && videoDurationRef.current > 0) {
        progressUpdaterRef.current.startTracking(video.currentTime)
      }
    }

    const handlePause = () => {
      onPauseRef.current()
      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }

    const handleTimeUpdate = () => {
      if (isSeekingRef.current) {
        return
      }

      const currentSecond = Math.floor(video.currentTime)
      if (currentSecond !== lastTimeUpdateRef.current) {
        lastTimeUpdateRef.current = currentSecond
        const actualTime = video.currentTime + (streamOffsetRef.current || 0)
        onTimeUpdateRef.current(actualTime)

        if (progressUpdaterRef.current && isPlayingRef.current) {
          progressUpdaterRef.current.updateCurrentTime(actualTime)
        }
      }
    }

    const handleEnded = () => {
      onEndedRef.current()
      if (progressUpdaterRef.current && videoDurationRef.current > 0) {
        progressUpdaterRef.current.stopTracking()
        progressUpdaterRef.current.updateCurrentTime(videoDurationRef.current)
        progressUpdaterRef.current.stopTracking()
      }
    }

    const handleWaiting = () => {
      onBufferingStartRef.current()
      stallStartRef.current = Date.now()
      recordStallRef.current()
    }

    const handleCanPlay = () => {
      if (stallStartRef.current > 0) {
        const stallDuration = Date.now() - stallStartRef.current
        if (stallDuration > 100) {
          onBufferingEndRef.current(stallDuration)
        }
        stallStartRef.current = 0
      } else {
        onBufferingEndRef.current(0)
      }
      ensureVideoUnmuted(video)
    }

    // 'playing' event is the most reliable indicator that playback has resumed
    // This catches cases where 'canplay' fires but buffering indicator stays stuck
    const handlePlaying = () => {
      if (stallStartRef.current > 0) {
        const stallDuration = Date.now() - stallStartRef.current
        onBufferingEndRef.current(stallDuration)
        stallStartRef.current = 0
      } else {
        // Always clear buffering state when actually playing
        onBufferingEndRef.current(0)
      }
    }

    const handleVolumeChange = () => {
      onVolumeChangeRef.current(video.volume, video.muted)
    }

    const handleFullscreenChange = () => {
      onFullscreenChangeRef.current(!!document.fullscreenElement)
    }

    const handlePiPEnter = () => {
      onPiPEnterRef.current()
    }

    const handlePiPExit = () => {
      onPiPExitRef.current()
    }

    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)
    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('waiting', handleWaiting)
    video.addEventListener('canplay', handleCanPlay)
    video.addEventListener('playing', handlePlaying)
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
      video.removeEventListener('playing', handlePlaying)
      video.removeEventListener('volumechange', handleVolumeChange)
      video.removeEventListener('enterpictureinpicture', handlePiPEnter)
      video.removeEventListener('leavepictureinpicture', handlePiPExit)
      document.removeEventListener('fullscreenchange', handleFullscreenChange)

      if (progressUpdaterRef.current) {
        progressUpdaterRef.current.stopTracking()
      }
    }
  // Note: All callbacks and changing values are stored in refs to avoid re-attaching event listeners
   
  }, [videoRef, mediaId, streamOffsetRef, isSeekingRef])

  return {
    lastTimeUpdateRef,
  }
}
