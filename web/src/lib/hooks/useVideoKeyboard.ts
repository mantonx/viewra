/**
 * useVideoKeyboard Hook
 * Handles keyboard shortcuts for video player controls.
 */

import { useEffect } from 'react'

export interface UseVideoKeyboardOptions {
  videoRef: React.RefObject<HTMLVideoElement | null>
  containerRef: React.RefObject<HTMLDivElement | null>
  videoDuration: number
  onToggleDebug: () => void
}

export const useVideoKeyboard = ({
  videoRef,
  containerRef,
  videoDuration,
  onToggleDebug,
}: UseVideoKeyboardOptions) => {
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
            containerRef.current?.requestFullscreen()
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
          onToggleDebug()
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
  }, [videoRef, containerRef, videoDuration, onToggleDebug])
}
