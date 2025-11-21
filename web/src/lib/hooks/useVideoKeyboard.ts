import { useEffect } from 'react'
import { logger } from '../utils/logger'

/**
 * Hook for handling keyboard shortcuts in the video player.
 * Supports play/pause, seeking, volume, fullscreen, and percentage-based seeking.
 */
export const useVideoKeyboard = (
  videoRef: React.RefObject<HTMLVideoElement>,
  videoContainerRef: React.RefObject<HTMLDivElement>,
  videoDuration: number
) => {
  useEffect(() => {
    const video = videoRef.current
    const container = videoContainerRef.current
    if (!video || !container) {return}

    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't trigger shortcuts if user is typing in an input field
      const target = e.target as HTMLElement
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') {
        return
      }

      switch (e.key.toLowerCase()) {
        // Play/Pause
        case ' ':
        case 'k':
          e.preventDefault()
          if (video.paused) {
            video.play()
          } else {
            video.pause()
          }
          break

        // Seek backward 10s
        case 'j':
        case 'arrowleft':
          e.preventDefault()
          video.currentTime = Math.max(0, video.currentTime - 10)
          logger.debug('Seeking backward 10s')
          break

        // Seek forward 10s
        case 'l':
        case 'arrowright':
          e.preventDefault()
          video.currentTime = Math.min(videoDuration, video.currentTime + 10)
          logger.debug('Seeking forward 10s')
          break

        // Volume up
        case 'arrowup':
          e.preventDefault()
          video.volume = Math.min(1, video.volume + 0.1)
          video.muted = false
          logger.debug('Volume up', { volume: video.volume })
          break

        // Volume down
        case 'arrowdown':
          e.preventDefault()
          video.volume = Math.max(0, video.volume - 0.1)
          logger.debug('Volume down', { volume: video.volume })
          break

        // Mute toggle
        case 'm':
          e.preventDefault()
          video.muted = !video.muted
          logger.debug('Mute toggled', { muted: video.muted })
          break

        // Fullscreen toggle
        case 'f':
          e.preventDefault()
          if (!document.fullscreenElement) {
            container.requestFullscreen().catch((err) => {
              logger.error('Failed to enter fullscreen', err)
            })
          } else {
            document.exitFullscreen()
          }
          break

        // Jump to percentage (0-9 keys)
        case '0':
        case '1':
        case '2':
        case '3':
        case '4':
        case '5':
        case '6':
        case '7':
        case '8':
        case '9': {
          e.preventDefault()
          const percentage = Number.parseInt(e.key, 10) / 10
          video.currentTime = videoDuration * percentage
          logger.debug(`Seeking to ${percentage * 100}%`)
          break
        }

        // Jump to start
        case 'home':
          e.preventDefault()
          video.currentTime = 0
          logger.debug('Seeking to start')
          break

        // Jump to end
        case 'end':
          e.preventDefault()
          video.currentTime = videoDuration
          logger.debug('Seeking to end')
          break

        default:
          break
      }
    }

    // Attach event listener to document for global keyboard shortcuts
    document.addEventListener('keydown', handleKeyDown)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [videoRef, videoContainerRef, videoDuration])
}
