import { useState, useRef, useEffect, useMemo } from 'react'
import { formatTime } from '@/lib/utils'
import { NerdMenu } from '../NerdMenu'
import { QualitySelector } from '../QualitySelector'
import { SpeedSelector } from '../SpeedSelector'
import { SubtitleSelector } from '../SubtitleSelector'
import { AudioSelector } from '../AudioSelector'
import type { VideoControlsProps } from './VideoControls.types'

// Format bandwidth to human readable string
const formatBandwidth = (bps: number): string => {
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(1)} Mbps`
  }
  return `${(bps / 1_000).toFixed(0)} kbps`
}

// Quality indicator component - shows current quality with Original badge when applicable
const QualityIndicator = ({
  currentQuality,
  currentBandwidth,
  availableQualities,
}: {
  currentQuality: number
  currentBandwidth: number | null
  availableQualities: { height: number; bandwidth: number }[]
}) => {
  // Find the original quality (highest resolution + highest bitrate at that resolution)
  const originalQuality = useMemo(() => {
    if (availableQualities.length === 0) return null
    const maxHeight = Math.max(...availableQualities.map(q => q.height))
    const atMaxHeight = availableQualities.filter(q => q.height === maxHeight)
    const maxBandwidth = Math.max(...atMaxHeight.map(q => q.bandwidth))
    return { height: maxHeight, bandwidth: maxBandwidth }
  }, [availableQualities])

  const isOriginal = originalQuality &&
    currentQuality === originalQuality.height &&
    currentBandwidth === originalQuality.bandwidth

  return (
    <div className="text-xs text-white/50 hidden lg:flex items-center gap-1.5 ml-2">
      <span className={`w-1.5 h-1.5 rounded-full ${isOriginal ? 'bg-amber-400' : 'bg-green-400'} animate-pulse`} />
      <span>
        {currentQuality}p
        {isOriginal && <span className="text-amber-400/80 ml-1">✦</span>}
        {currentBandwidth && (
          <span className="text-white/40 ml-1">
            ({formatBandwidth(currentBandwidth)})
          </span>
        )}
      </span>
    </div>
  )
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
  currentBandwidth,
  autoMode,
  availableAudioTracks,
  currentAudioTrack,
  availableSubtitles,
  currentSubtitle,
  playbackSpeed,
  metadata,
  showStats,
  onPlayPause,
  onSeek,
  onVolumeChange,
  onMuteToggle,
  onFullscreenToggle,
  onPiPToggle,
  onQualityChange,
  onAutoToggle,
  onAudioTrackChange,
  onSubtitleChange,
  onSpeedChange,
  onSkip,
  onToggleStats,
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
      {/* Top metadata bar with gradient scrim */}
      {metadata && (
        <div
          className={`absolute top-0 left-0 right-0 transition-opacity duration-300 ${
            showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
          }`}
        >
          {/* Gradient scrim for legibility */}
          <div className="absolute inset-0 bg-linear-to-b from-black/70 via-black/40 to-transparent pointer-events-none" />

          <div className="relative px-6 pt-4 pb-12">
            <div className="flex items-start gap-4">
              {/* Poster artwork */}
              {metadata.posterUrl && (
                <img
                  src={metadata.posterUrl}
                  alt={metadata.title}
                  className="w-16 h-24 object-cover rounded-lg shadow-xl ring-1 ring-white/10"
                />
              )}
              {/* Title and subtitle */}
              <div className="flex-1">
                <h2 className="text-white text-xl font-semibold drop-shadow-lg">{metadata.title}</h2>
                {metadata.subtitle && (
                  <p className="text-white/80 text-sm mt-1 drop-shadow-md">{metadata.subtitle}</p>
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
        {/* Gradient scrim for legibility */}
        <div className="absolute inset-0 bg-linear-to-t from-black/80 via-black/50 to-transparent pointer-events-none" />

        {/* Controls container */}
        <div className="relative px-6 pb-6 pt-16">
        {/* Timeline */}
        <div className="mb-3">
          <div
            ref={timelineRef}
            className="relative h-1.5 bg-white/20 rounded-full cursor-pointer group hover:h-2 transition-all touch-none backdrop-blur-sm"
            onMouseMove={handleTimelineMouseMove}
            onMouseLeave={() => setHoverTime(null)}
            onMouseDown={(e) => {
              setIsDragging(true)
              handleTimelineInteraction(e)
            }}
            onMouseUp={() => setIsDragging(false)}
          >
            {/* Buffered indicator (subtle) */}
            <div className="absolute h-full bg-white/10 rounded-full pointer-events-none" style={{ width: '100%' }} />

            {/* Progress bar with glow */}
            <div
              className="absolute h-full bg-primary-500 rounded-full pointer-events-none shadow-[0_0_12px_rgba(59,130,246,0.5)]"
              style={{ width: `${progress}%` }}
            />

            {/* Hover preview tooltip */}
            {hoverTime !== null && (
              <div
                className="absolute bottom-full mb-3 px-3 py-1.5 bg-black/90 backdrop-blur-md text-white text-xs font-medium rounded-lg whitespace-nowrap pointer-events-none border border-white/10 shadow-xl"
                style={{
                  left: `${(hoverTime / duration) * 100}%`,
                  transform: 'translateX(-50%)',
                }}
              >
                {formatTime(hoverTime)}
              </div>
            )}

            {/* Scrubber with glow effect */}
            <div
              className={`absolute w-4 h-4 bg-white rounded-full shadow-[0_0_0_4px_rgba(59,130,246,0.3),0_2px_8px_rgba(0,0,0,0.3)] transition-all pointer-events-none group-hover:scale-125 group-hover:shadow-[0_0_0_6px_rgba(59,130,246,0.4),0_2px_12px_rgba(0,0,0,0.4)] ${
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
              className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
              aria-label="Rewind 10 seconds"
            >
              <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                <path d="M11.99 5V1l-5 5 5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6h-2c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8zm-1.1 11h-.85v-3.26l-1.01.31v-.69l1.77-.63h.09V16zm4.28-1.76c0 .32-.03.6-.1.82s-.17.42-.29.57-.28.26-.45.33-.37.1-.59.10-.41-.03-.59-.1-.33-.18-.46-.33-.23-.34-.3-.57-.11-.5-.11-.82v-.74c0-.32.03-.6.1-.82s.17-.42.29-.57.28-.26.45-.33.37-.1.59-.1.41.03.59.1.33.18.46.33.23.34.3.57.11.5.11.82v.74zm-.85-.86c0-.19-.01-.35-.04-.48s-.07-.23-.12-.31-.11-.14-.19-.17-.16-.05-.25-.05-.18.02-.25.05-.14.09-.19.17-.09.18-.12.31-.04.29-.04.48v.97c0 .19.01.35.04.48s.07.24.12.32.11.14.19.17.16.05.25.05.18-.02.25-.05.14-.09.19-.17.09-.19.11-.32.04-.29.04-.48v-.97z" />
              </svg>
            </button>

            {/* Play/Pause */}
            <button
              onClick={onPlayPause}
              className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
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
              className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
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
                className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
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
                  className="w-full accent-primary-500 cursor-pointer"
                />
              </div>
            </div>

            {/* Time display */}
            <div className="text-sm font-medium hidden sm:block">
              <span>{formatTime(currentTime)} / {formatTime(duration)}</span>
              {getEndTime() && (
                <span className="text-white/60 ml-2 hidden md:inline">• Ends at {getEndTime()}</span>
              )}
            </div>

            {/* Current quality indicator - subtle display of what's playing */}
            {currentQuality && currentQuality > 0 && (
              <QualityIndicator
                currentQuality={currentQuality}
                currentBandwidth={currentBandwidth ?? null}
                availableQualities={availableQualities}
              />
            )}
          </div>

          {/* Right controls */}
          <div className="flex items-center gap-1 sm:gap-2">
            {/* Speed selector */}
            <SpeedSelector playbackSpeed={playbackSpeed} onSpeedChange={onSpeedChange} />

            {/* Quality selector */}
            {availableQualities.length > 0 && (
              <QualitySelector
                currentQuality={currentQuality}
                currentBandwidth={currentBandwidth}
                availableQualities={availableQualities}
                autoMode={autoMode}
                onQualityChange={onQualityChange}
                onAutoToggle={onAutoToggle}
              />
            )}

            {/* Subtitle selector */}
            {availableSubtitles.length > 0 && (
              <SubtitleSelector
                availableSubtitles={availableSubtitles}
                currentSubtitle={currentSubtitle}
                onSubtitleChange={onSubtitleChange}
              />
            )}

            {/* Audio track selector */}
            {availableAudioTracks.length > 1 && (
              <AudioSelector
                availableAudioTracks={availableAudioTracks}
                currentAudioTrack={currentAudioTrack}
                onAudioTrackChange={onAudioTrackChange}
              />
            )}

            {/* Nerd Menu */}
            <NerdMenu showStats={showStats} onToggleStats={onToggleStats} />

            {/* Picture-in-Picture */}
            {document.pictureInPictureEnabled && (
              <button
                onClick={onPiPToggle}
                className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
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
              className="hover:bg-white/20 p-2 rounded-lg transition-colors cursor-pointer"
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
