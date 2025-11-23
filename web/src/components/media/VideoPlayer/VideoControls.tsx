import { useState, useRef, useEffect } from 'react'
import { formatTime } from '@/lib/utils'

interface MediaMetadata {
  title: string
  subtitle?: string
  posterUrl?: string
}

interface VideoControlsProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  isPlaying: boolean
  currentTime: number
  duration: number
  volume: number
  isMuted: boolean
  isFullscreen: boolean
  isPiP: boolean
  availableQualities: Array<{ height: number; bandwidth: number }>
  currentQuality: number | null
  availableAudioTracks: Array<{ id: number; name: string; language: string }>
  currentAudioTrack: number
  playbackSpeed: number
  metadata?: MediaMetadata
  onPlayPause: () => void
  onSeek: (time: number) => void
  onVolumeChange: (volume: number) => void
  onMuteToggle: () => void
  onFullscreenToggle: () => void
  onPiPToggle: () => void
  onQualityChange: (height: number) => void
  onAudioTrackChange: (trackId: number) => void
  onSpeedChange: (speed: number) => void
  onSkip: (seconds: number) => void
}

export const VideoControls = ({
  videoRef,
  isPlaying,
  currentTime,
  duration,
  volume,
  isMuted,
  isFullscreen,
  isPiP,
  availableQualities,
  currentQuality,
  availableAudioTracks,
  currentAudioTrack,
  playbackSpeed,
  metadata,
  onPlayPause,
  onSeek,
  onVolumeChange,
  onMuteToggle,
  onFullscreenToggle,
  onPiPToggle,
  onQualityChange,
  onAudioTrackChange,
  onSpeedChange,
  onSkip,
}: VideoControlsProps) => {
  const [showControls, setShowControls] = useState(true)
  const [isDragging, setIsDragging] = useState(false)
  const [hoverTime, setHoverTime] = useState<number | null>(null)
  const timelineRef = useRef<HTMLDivElement>(null)
  const hideTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Auto-hide controls after inactivity (3s when playing, 30s when paused)
  useEffect(() => {
    const handleMouseMove = () => {
      setShowControls(true)

      // Clear any existing timeout
      if (hideTimeoutRef.current) {
        clearTimeout(hideTimeoutRef.current)
      }

      // Only set timeout to hide controls when playing
      // When paused, controls stay visible indefinitely
      if (isPlaying) {
        hideTimeoutRef.current = setTimeout(() => {
          if (!isDragging) {
            setShowControls(false)
          }
        }, 3000)
      }
    }

    const handleMouseLeave = () => {
      // Only hide controls on mouse leave if video is playing
      if (isPlaying && !isDragging) {
        setShowControls(false)
      }
      // When paused, keep controls visible even after mouse leaves
    }

    const container = videoRef.current?.parentElement
    if (container) {
      container.addEventListener('mousemove', handleMouseMove)
      container.addEventListener('mouseleave', handleMouseLeave)
    }

    return () => {
      if (container) {
        container.removeEventListener('mousemove', handleMouseMove)
        container.removeEventListener('mouseleave', handleMouseLeave)
      }
      if (hideTimeoutRef.current) {
        clearTimeout(hideTimeoutRef.current)
      }
    }
  }, [isPlaying, isDragging, videoRef])

  // Handle timeline click/drag
  const handleTimelineInteraction = (e: React.MouseEvent<HTMLDivElement>) => {
    const timeline = timelineRef.current
    if (!timeline || duration === 0) {
      return
    }

    const rect = timeline.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    const time = pos * duration

    onSeek(Math.max(0, Math.min(duration, time)))
  }

  const handleTimelineMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    const timeline = timelineRef.current
    if (!timeline || duration === 0) {
      return
    }

    const rect = timeline.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    const time = pos * duration

    setHoverTime(Math.max(0, Math.min(duration, time)))
  }

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0

  // Calculate end time
  const getEndTime = () => {
    if (duration === 0 || currentTime === 0) {
      return null
    }
    const remainingSeconds = duration - currentTime
    const now = new Date()
    const endTime = new Date(now.getTime() + remainingSeconds * 1000)
    return endTime.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
  }

  return (
    <>
      {/* Top metadata bar */}
      {metadata && (
        <div
          className={`absolute top-0 left-0 right-0 transition-opacity duration-300 ${
            showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
          }`}
        >
          <div className="bg-gradient-to-b from-black/80 to-transparent p-6">
            <div className="flex items-start gap-4">
              {/* Poster artwork */}
              {metadata.posterUrl && (
                <img
                  src={metadata.posterUrl}
                  alt={metadata.title}
                  className="w-16 h-24 object-cover rounded shadow-lg"
                />
              )}
              {/* Title and subtitle */}
              <div className="flex-1">
                <h2 className="text-white text-xl font-semibold">{metadata.title}</h2>
                {metadata.subtitle && (
                  <p className="text-white/80 text-sm mt-1">{metadata.subtitle}</p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Bottom controls */}
      <div
        className={`absolute bottom-0 left-0 right-0 transition-opacity duration-300 ${
          showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
      >
        {/* Gradient overlay for readability */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/40 to-transparent pointer-events-none" />

      {/* Controls container */}
      <div className="relative px-4 pb-4 pt-8">
        {/* Timeline */}
        <div className="mb-3">
          <div
            ref={timelineRef}
            className="relative h-1 bg-white/30 rounded-full cursor-pointer group hover:h-2 transition-all"
            onClick={handleTimelineInteraction}
            onMouseMove={handleTimelineMouseMove}
            onMouseLeave={() => setHoverTime(null)}
            onMouseDown={(e) => {
              setIsDragging(true)
              handleTimelineInteraction(e)
            }}
            onMouseUp={() => setIsDragging(false)}
          >
            {/* Progress bar */}
            <div
              className="absolute h-full bg-indigo-500 rounded-full pointer-events-none"
              style={{ width: `${progress}%` }}
            />

            {/* Hover preview */}
            {hoverTime !== null && (
              <div
                className="absolute bottom-full mb-2 px-2 py-1 bg-black/90 text-white text-xs rounded whitespace-nowrap pointer-events-none"
                style={{
                  left: `${(hoverTime / duration) * 100}%`,
                  transform: 'translateX(-50%)',
                }}
              >
                {formatTime(hoverTime)}
              </div>
            )}

            {/* Scrubber */}
            <div
              className={`absolute w-4 h-4 bg-indigo-500 rounded-full shadow-lg transition-all pointer-events-none ${
                showControls ? 'opacity-100' : 'opacity-0'
              }`}
              style={{ left: `${progress}%`, top: '50%', transform: 'translate(-50%, -50%)' }}
            />
          </div>
        </div>

        {/* Control buttons */}
        <div className="flex items-center justify-between text-white">
          {/* Left controls */}
          <div className="flex items-center gap-2">
            {/* Rewind 10 seconds */}
            <button
              onClick={() => onSkip(-10)}
              className="hover:bg-white/20 p-2 rounded-lg transition-colors"
              aria-label="Rewind 10 seconds"
            >
              <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                <path d="M11.99 5V1l-5 5 5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6h-2c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8zm-1.1 11h-.85v-3.26l-1.01.31v-.69l1.77-.63h.09V16zm4.28-1.76c0 .32-.03.6-.1.82s-.17.42-.29.57-.28.26-.45.33-.37.1-.59.10-.41-.03-.59-.1-.33-.18-.46-.33-.23-.34-.3-.57-.11-.5-.11-.82v-.74c0-.32.03-.6.1-.82s.17-.42.29-.57.28-.26.45-.33.37-.1.59-.1.41.03.59.1.33.18.46.33.23.34.3.57.11.5.11.82v.74zm-.85-.86c0-.19-.01-.35-.04-.48s-.07-.23-.12-.31-.11-.14-.19-.17-.16-.05-.25-.05-.18.02-.25.05-.14.09-.19.17-.09.18-.12.31-.04.29-.04.48v.97c0 .19.01.35.04.48s.07.24.12.32.11.14.19.17.16.05.25.05.18-.02.25-.05.14-.09.19-.17.09-.19.11-.32.04-.29.04-.48v-.97z" />
              </svg>
            </button>

            {/* Play/Pause */}
            <button
              onClick={onPlayPause}
              className="hover:bg-white/20 p-2 rounded-lg transition-colors"
              aria-label={isPlaying ? 'Pause' : 'Play'}
            >
              {isPlaying ? (
                <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
                </svg>
              ) : (
                <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
              )}
            </button>

            {/* Skip 10 seconds */}
            <button
              onClick={() => onSkip(10)}
              className="hover:bg-white/20 p-2 rounded-lg transition-colors"
              aria-label="Skip 10 seconds"
            >
              <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                <path d="M4 13c0 4.42 3.58 8 8 8s8-3.58 8-8h-2c0 3.31-2.69 6-6 6s-6-2.69-6-6 2.69-6 6-6v4l5-5-5-5v4c-4.42 0-8 3.58-8 8zm7.9 3h-.85v-3.26l-1.01.31v-.69l1.77-.63h.09V16zm4.28-1.76c0 .32-.03.6-.1.82s-.17.42-.29.57-.28.26-.45.33-.37.1-.59.1-.41-.03-.59-.1-.33-.18-.46-.33-.23-.34-.3-.57-.11-.5-.11-.82v-.74c0-.32.03-.6.1-.82s.17-.42.29-.57.28-.26.45-.33.37-.1.59-.1.41.03.59.1.33.18.46.33.23.34.3.57.11.5.11.82v.74zm-.85-.86c0-.19-.01-.35-.04-.48s-.07-.23-.12-.31-.11-.14-.19-.17-.16-.05-.25-.05-.18.02-.25.05-.14.09-.19.17-.09.18-.12.31-.04.29-.04.48v.97c0 .19.01.35.04.48s.07.24.12.32.11.14.19.17.16.05.25.05.18-.02.25-.05.14-.09.19-.17.09-.19.11-.32.04-.29.04-.48v-.97z" />
              </svg>
            </button>

            {/* Volume */}
            <div className="flex items-center gap-2 group">
              <button
                onClick={onMuteToggle}
                className="hover:bg-white/20 p-2 rounded-lg transition-colors"
                aria-label={isMuted ? 'Unmute' : 'Mute'}
              >
                {isMuted || volume === 0 ? (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z" />
                  </svg>
                ) : volume < 0.5 ? (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M7 9v6h4l5 5V4l-5 5H7z" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02z" />
                  </svg>
                )}
              </button>

              {/* Volume slider (visible on hover) */}
              <div className="w-0 opacity-0 group-hover:w-20 group-hover:opacity-100 transition-all overflow-hidden">
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={volume}
                  onChange={(e) => onVolumeChange(parseFloat(e.target.value))}
                  className="w-full accent-indigo-500"
                />
              </div>
            </div>

            {/* Time display */}
            <div className="text-sm font-medium">
              <span>{formatTime(currentTime)} / {formatTime(duration)}</span>
              {getEndTime() && (
                <span className="text-white/60 ml-2">• Ends at {getEndTime()}</span>
              )}
            </div>
          </div>

          {/* Right controls */}
          <div className="flex items-center gap-2">
            {/* Speed selector */}
            <select
              value={playbackSpeed}
              onChange={(e) => onSpeedChange(parseFloat(e.target.value))}
              className="bg-white/10 backdrop-blur-sm text-white text-sm rounded-md px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-indigo-500/50"
              style={{ minWidth: '80px' }}
            >
              <option value={0.25} className="bg-neutral-900 dark:bg-neutral-900">
                0.25x
              </option>
              <option value={0.5} className="bg-neutral-900 dark:bg-neutral-900">
                0.5x
              </option>
              <option value={0.75} className="bg-neutral-900 dark:bg-neutral-900">
                0.75x
              </option>
              <option value={1} className="bg-neutral-900 dark:bg-neutral-900">
                Normal
              </option>
              <option value={1.25} className="bg-neutral-900 dark:bg-neutral-900">
                1.25x
              </option>
              <option value={1.5} className="bg-neutral-900 dark:bg-neutral-900">
                1.5x
              </option>
              <option value={1.75} className="bg-neutral-900 dark:bg-neutral-900">
                1.75x
              </option>
              <option value={2} className="bg-neutral-900 dark:bg-neutral-900">
                2x
              </option>
            </select>

            {/* Quality selector */}
            {availableQualities.length > 0 && (
              <select
                value={currentQuality || 0}
                onChange={(e) => onQualityChange(Number(e.target.value))}
                className="bg-white/10 backdrop-blur-sm text-white text-sm rounded-md px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-indigo-500/50"
                style={{ minWidth: '90px' }}
              >
                <option value={0} className="bg-neutral-900 dark:bg-neutral-900">
                  Auto
                </option>
                {availableQualities.map((quality) => (
                  <option key={quality.height} value={quality.height} className="bg-neutral-900 dark:bg-neutral-900">
                    {quality.height}p
                  </option>
                ))}
              </select>
            )}

            {/* Audio track selector */}
            {availableAudioTracks.length > 1 && (
              <select
                value={currentAudioTrack}
                onChange={(e) => onAudioTrackChange(Number(e.target.value))}
                className="bg-white/10 backdrop-blur-sm text-white text-sm rounded-md px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-indigo-500/50"
                style={{ minWidth: '110px' }}
                aria-label="Audio track"
              >
                {availableAudioTracks.map((track) => (
                  <option key={track.id} value={track.id} className="bg-neutral-900 dark:bg-neutral-900">
                    {track.name} ({track.language})
                  </option>
                ))}
              </select>
            )}

            {/* Picture-in-Picture */}
            {document.pictureInPictureEnabled && (
              <button
                onClick={onPiPToggle}
                className="hover:bg-white/20 p-2 rounded-lg transition-colors"
                aria-label={isPiP ? 'Exit Picture-in-Picture' : 'Picture-in-Picture'}
              >
                {isPiP ? (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M19 7h-8v6h8V7zm2-4H3c-1.1 0-2 .9-2 2v14c0 1.1.9 1.98 2 1.98h18c1.1 0 2-.88 2-1.98V5c0-1.1-.9-2-2-2zm0 16.01H3V4.98h18v14.03z" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M19 11h-8v6h8v-6zm4 8V4.98C23 3.88 22.1 3 21 3H3c-1.1 0-2 .88-2 1.98V19c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2zm-2 .02H3V4.97h18v14.05z" />
                  </svg>
                )}
              </button>
            )}

            {/* Fullscreen */}
            <button
              onClick={onFullscreenToggle}
              className="hover:bg-white/20 p-2 rounded-lg transition-colors"
              aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
            >
              {isFullscreen ? (
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z" />
                </svg>
              ) : (
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z" />
                </svg>
              )}
            </button>
          </div>
        </div>
      </div>
      </div>
    </>
  )
}
