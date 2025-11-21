/**
 * Utility functions for video element manipulation.
 */

/**
 * Ensures the video element is unmuted with full volume.
 * Useful for autoplay scenarios where browser policies may mute the video.
 */
export const ensureVideoUnmuted = (video: HTMLVideoElement): void => {
  video.muted = false
  video.volume = 1.0
}

/**
 * Sets the initial playback position for a video element.
 * Can optionally wait for metadata to be loaded before seeking.
 */
export const setInitialPosition = (
  video: HTMLVideoElement,
  position: number,
  waitForMetadata = false
): void => {
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

/**
 * Formats time in seconds to MM:SS or HH:MM:SS format.
 */
export const formatTime = (seconds: number): string => {
  if (Number.isNaN(seconds) || !Number.isFinite(seconds)) {
    return '0:00'
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  return `${minutes}:${secs.toString().padStart(2, '0')}`
}
