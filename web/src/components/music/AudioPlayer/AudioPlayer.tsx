import { useAudioPlayer } from '@/lib/contexts/AudioPlayerContext'
import { useContext, useEffect, useRef, useState } from 'react'
import { text, bg, glassStyles, animation, animationStyles } from '@/styles/semantic'
import { ThemeContext } from '@/contexts/ThemeContext'
import { cn } from '@/lib/utils'
import type { AudioPlayerProps } from './AudioPlayer.types'
import { MediaPoster } from '@/components/media/MediaPoster'
import { QueuePanel } from './QueuePanel'
import { KeyboardShortcuts } from './KeyboardShortcuts'
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  Shuffle,
  Repeat,
  Repeat1,
  Volume2,
  Volume1,
  VolumeX,
  Loader2,
  ListMusic,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'

const formatTime = (seconds: number): string => {
  if (!isFinite(seconds)) {
    return '0:00'
  }
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const AudioPlayer = ({ className = '' }: AudioPlayerProps) => {
  const themeContext = useContext(ThemeContext)
  const theme = themeContext?.theme || 'light'

  const {
    currentTrack,
    isPlaying,
    isLoading,
    volume,
    isMuted,
    currentTime,
    duration,
    repeatMode,
    isShuffle,
    isMinimized,
    visibility,
    togglePlayPause,
    playNext,
    playPrevious,
    seek,
    setVolume,
    toggleMute,
    toggleRepeat,
    toggleShuffle,
    toggleMinimized,
    setMinimized,
    setVisibility,
  } = useAudioPlayer()

  const [isSeeking, setIsSeeking] = useState(false)
  const [seekPosition, setSeekPosition] = useState(0)
  const [showVolumeSlider, setShowVolumeSlider] = useState(false)
  const [showQueue, setShowQueue] = useState(false)
  const [showShortcuts, setShowShortcuts] = useState(false)
  const [showAlbumArt, setShowAlbumArt] = useState(false)
  const volumeRef = useRef<HTMLDivElement>(null)

  // Close volume slider when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (volumeRef.current && !volumeRef.current.contains(event.target as Node)) {
        setShowVolumeSlider(false)
      }
    }

    if (showVolumeSlider) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showVolumeSlider])

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      // Only handle if not typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      // Handle Escape key for modals
      if (e.key === 'Escape') {
        if (showAlbumArt) {
          e.preventDefault()
          setShowAlbumArt(false)
          return
        }
      }

      switch (e.key) {
        case ' ':
          e.preventDefault()
          togglePlayPause()
          break
        case 'ArrowLeft':
          e.preventDefault()
          seek(Math.max(0, currentTime - 5))
          break
        case 'ArrowRight':
          e.preventDefault()
          seek(Math.min(duration, currentTime + 5))
          break
        case 'm':
        case 'M':
          e.preventDefault()
          toggleMute()
          break
        case 'ArrowUp':
          e.preventDefault()
          setVolume(Math.min(1, volume + 0.1))
          break
        case 'ArrowDown':
          e.preventDefault()
          setVolume(Math.max(0, volume - 0.1))
          break
        case '?':
          e.preventDefault()
          setShowShortcuts(true)
          break
      }
    }

    document.addEventListener('keydown', handleKeyPress)
    return () => document.removeEventListener('keydown', handleKeyPress)
  }, [currentTime, duration, volume, showAlbumArt, togglePlayPause, toggleMute, seek, setVolume])

  // Don't render if no track or visibility is hidden
  if (!currentTrack || visibility === 'hidden') {
    return null
  }

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0
  const displayTime = isSeeking ? seekPosition : currentTime

  const handleSeekStart = () => {
    setIsSeeking(true)
    setSeekPosition(currentTime)
  }

  const handleSeekChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = parseFloat(e.target.value)
    setSeekPosition(newTime)
  }

  const handleSeekEnd = () => {
    seek(seekPosition)
    setIsSeeking(false)
  }

  const handleProgressClick = (e: React.MouseEvent<HTMLInputElement>) => {
    // Allow clicking anywhere on the progress bar to seek
    const input = e.currentTarget
    const rect = input.getBoundingClientRect()
    const clickX = e.clientX - rect.left
    const percentage = clickX / rect.width
    const newTime = percentage * duration
    seek(Math.max(0, Math.min(duration, newTime)))
  }

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setVolume(parseFloat(e.target.value))
  }

  const getRepeatIcon = () => {
    switch (repeatMode) {
      case 'off':
        return <Repeat size={18} />
      case 'all':
        return <Repeat size={18} />
      case 'one':
        return <Repeat1 size={18} />
    }
  }

  const getVolumeIcon = () => {
    if (isMuted || volume === 0) {
      return <VolumeX size={18} />
    }
    if (volume < 0.5) {
      return <Volume1 size={18} />
    }
    return <Volume2 size={18} />
  }

  // Minimized player view - full width translucent bar
  if (isMinimized) {
    return (
      <div
        className={cn(
          'fixed bottom-0 left-0 right-0 z-50 border-t shadow-lg',
          bg.elevated,
          'border-neutral-200 dark:border-neutral-800',
          animation.classes.slideUpMedium,
          className
        )}
        style={{
          ...glassStyles.medium(theme === 'dark'),
          ...animationStyles.slideUp(animation.duration.moderate),
        }}
      >
        {/* Progress bar at top */}
        <div className="absolute top-0 left-0 right-0 h-1 bg-neutral-200 dark:bg-neutral-700">
          <div
            className="h-full bg-primary-500 transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>

        <div className="max-w-screen-2xl mx-auto px-4 py-2">
          <div className="flex items-center gap-3">
            {/* Album art */}
            <div
              className={cn(
                'w-10 h-10 rounded overflow-hidden shrink-0',
                bg.tertiary
              )}
            >
              <MediaPoster
                mediaId={currentTrack.id}
                mediaType="media"
                alt={currentTrack.album || currentTrack.title}
                className="w-full h-full object-cover"
                preset="thumb"
                aspectRatio="square"
                fallbackIcon="🎵"
              />
            </div>

            {/* Track info */}
            <div className="flex-1 min-w-0">
              <p className={cn('text-sm font-medium truncate', text.primary)}>
                {currentTrack.title}
              </p>
              <p className={cn('text-xs truncate', text.tertiary)}>
                {currentTrack.artist || 'Unknown Artist'}
              </p>
            </div>

            {/* Playback controls */}
            <div className="flex items-center gap-1">
              {/* Previous */}
              <button
                onClick={playPrevious}
                className={cn(
                  'p-2 rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  text.tertiary
                )}
                title="Previous"
                aria-label="Previous track"
              >
                <SkipBack size={18} />
              </button>

              {/* Play/Pause */}
              <button
                onClick={togglePlayPause}
                className={cn(
                  'p-2 min-h-10 min-w-10 flex items-center justify-center bg-primary-600 rounded-full',
                  'hover:bg-primary-700 text-white cursor-pointer',
                  animation.button.subtle
                )}
                title={isLoading ? 'Loading...' : isPlaying ? 'Pause' : 'Play'}
                aria-label={isLoading ? 'Loading' : isPlaying ? 'Pause' : 'Play'}
              >
                {isLoading ? (
                  <Loader2 size={18} className="animate-spin" />
                ) : isPlaying ? (
                  <Pause size={18} />
                ) : (
                  <Play size={18} className="ml-0.5" />
                )}
              </button>

              {/* Next */}
              <button
                onClick={playNext}
                className={cn(
                  'p-2 rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  text.tertiary
                )}
                title="Next"
                aria-label="Next track"
              >
                <SkipForward size={18} />
              </button>
            </div>

            {/* Time display */}
            <div className={cn('text-xs tabular-nums hidden sm:block', text.tertiary)}>
              {formatTime(currentTime)} / {formatTime(duration)}
            </div>

            {/* Expand button */}
            <button
              onClick={() => {
                setMinimized(false)
                setVisibility('expanded')
              }}
              className={cn(
                'p-2 rounded cursor-pointer',
                animation.button.pronounced,
                bg.hover.default,
                text.tertiary
              )}
              title="Expand player"
              aria-label="Expand player"
            >
              <ChevronUp size={18} className="transition-transform hover:-translate-y-0.5" />
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <>
      <div
        className={cn(
          'fixed bottom-0 left-0 right-0 z-50 border-t',
          bg.elevated,
          'border-neutral-200 dark:border-neutral-800',
          'shadow-2xl',
          animation.classes.slideUpLarge,
          className
        )}
        style={{
          ...glassStyles.medium(theme === 'dark'),
          ...animationStyles.slideUp(animation.duration.slow),
        }}
      >
        <div className="max-w-screen-2xl mx-auto px-6 py-3">
          {/* Progress bar */}
          <div className="mb-3">
            <input
              type="range"
              min="0"
              max={duration || 0}
              value={displayTime}
              onChange={handleSeekChange}
              onClick={handleProgressClick}
              onMouseDown={handleSeekStart}
              onMouseUp={handleSeekEnd}
              onTouchStart={handleSeekStart}
              onTouchEnd={handleSeekEnd}
              className={cn(
                'w-full h-1 rounded-lg appearance-none cursor-pointer slider',
                'bg-neutral-200 dark:bg-neutral-700'
              )}
              style={{
                background: `linear-gradient(to right, var(--color-primary-500) 0%, var(--color-primary-500) ${progress}%, ${theme === 'dark' ? 'var(--color-neutral-700)' : 'var(--color-neutral-200)'} ${progress}%, ${theme === 'dark' ? 'var(--color-neutral-700)' : 'var(--color-neutral-200)'} 100%)`,
              }}
            />
            <div className={cn('flex justify-between text-xs mt-1', text.tertiary)}>
              <span>{formatTime(displayTime)}</span>
              <span>{formatTime(duration)}</span>
            </div>
          </div>

          <div className="flex items-center justify-between gap-4">
            {/* Album art and track info */}
            <div className="flex items-center gap-4 flex-1 min-w-0">
              {/* Album art thumbnail - larger and clickable */}
              <button
                onClick={() => setShowAlbumArt(true)}
                className={cn(
                  'w-20 h-20 rounded-lg overflow-hidden shrink-0',
                  'transition-transform hover:scale-105 cursor-pointer',
                  'shadow-lg',
                  bg.tertiary
                )}
                title="Click to view album art"
                aria-label="View album art in full size"
              >
                <MediaPoster
                  mediaId={currentTrack.id}
                  mediaType="media"
                  alt={currentTrack.album || currentTrack.title}
                  className="w-full h-full object-cover"
                  preset="medium"
                  aspectRatio="square"
                  fallbackIcon="🎵"
                />
              </button>

              {/* Track info */}
              <div className="flex-1 min-w-0">
                <h4 className={cn('font-semibold text-sm truncate', text.primary)}>
                  {currentTrack.title}
                </h4>
                <p className={cn('text-xs truncate', text.tertiary)}>
                  {currentTrack.artist || 'Unknown Artist'}
                  {currentTrack.album && ` • ${currentTrack.album}`}
                </p>
              </div>
            </div>

            {/* Playback controls */}
            <div className="flex items-center gap-3">
              {/* Shuffle */}
              <button
                onClick={toggleShuffle}
                className={cn(
                  'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  isShuffle ? 'text-primary-500' : text.tertiary
                )}
                title="Shuffle"
                aria-label="Toggle shuffle"
              >
                <Shuffle size={18} />
              </button>

              {/* Previous */}
              <button
                onClick={playPrevious}
                className={cn(
                  'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  text.primary
                )}
                title="Previous track"
                aria-label="Previous track"
              >
                <SkipBack size={20} />
              </button>

              {/* Play/Pause */}
              <button
                onClick={togglePlayPause}
                className={cn(
                  'p-3 min-h-[52px] min-w-[52px] flex items-center justify-center bg-primary-600 rounded-full',
                  'hover:bg-primary-700 text-white cursor-pointer',
                  animation.button.subtle
                )}
                title={isLoading ? 'Loading...' : isPlaying ? 'Pause' : 'Play'}
                aria-label={isLoading ? 'Loading' : isPlaying ? 'Pause' : 'Play'}
                disabled={isLoading}
              >
                {isLoading ? (
                  <Loader2 size={24} className="animate-spin" />
                ) : isPlaying ? (
                  <Pause size={24} />
                ) : (
                  <Play size={24} className="ml-0.5" />
                )}
              </button>

              {/* Next */}
              <button
                onClick={playNext}
                className={cn(
                  'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  text.primary
                )}
                title="Next track"
                aria-label="Next track"
              >
                <SkipForward size={20} />
              </button>

              {/* Repeat */}
              <button
                onClick={toggleRepeat}
                className={cn(
                  'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  repeatMode !== 'off' ? 'text-primary-500' : text.tertiary
                )}
                title={`Repeat: ${repeatMode}`}
                aria-label="Toggle repeat"
              >
                {getRepeatIcon()}
              </button>
            </div>

            {/* Queue button */}
            <button
              onClick={() => setShowQueue(!showQueue)}
              className={cn(
                'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                animation.button.pronounced,
                bg.hover.default,
                showQueue ? 'text-primary-500' : text.tertiary
              )}
              title="Queue"
              aria-label="Toggle queue"
            >
              <ListMusic size={18} />
            </button>

            {/* Volume control */}
            <div className="flex items-center gap-2 relative" ref={volumeRef}>
              <button
                onClick={toggleMute}
                className={cn(
                  'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                  animation.button.pronounced,
                  bg.hover.default,
                  isMuted ? 'text-primary-500' : text.tertiary
                )}
                title={isMuted ? 'Unmute (M)' : 'Mute (M)'}
                aria-label={isMuted ? 'Unmute' : 'Mute'}
              >
                {getVolumeIcon()}
              </button>
              <button
                onClick={() => setShowVolumeSlider(!showVolumeSlider)}
                className={cn(
                  'text-xs px-2 py-1 rounded cursor-pointer',
                  animation.button.subtle,
                  bg.hover.default,
                  text.tertiary
                )}
                title="Volume slider (↑↓)"
                aria-label="Toggle volume slider"
              >
                {Math.round((isMuted ? 0 : volume) * 100)}%
              </button>

              {showVolumeSlider && (
                <div
                  className={cn(
                    'absolute bottom-full right-0 mb-2 rounded-xl shadow-2xl p-4 border',
                    animation.classes.slideUpSmall,
                    'bg-neutral-900/95 backdrop-blur-xl',
                    'border-white/10'
                  )}
                  style={animationStyles.popup()}
                >
                  <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    value={volume}
                    onChange={handleVolumeChange}
                    className="w-24 h-1.5 rounded-full appearance-none cursor-pointer volume-slider"
                    style={{
                      background: `linear-gradient(to right, var(--color-primary-500) 0%, var(--color-primary-500) ${volume * 100}%, rgba(255,255,255,0.2) ${volume * 100}%, rgba(255,255,255,0.2) 100%)`,
                    }}
                    aria-label="Volume slider"
                  />
                  <div className="text-xs text-center mt-2 text-neutral-400 font-medium">
                    {Math.round(volume * 100)}%
                  </div>
                </div>
              )}
            </div>

            {/* Minimize button */}
            <button
              onClick={() => {
                toggleMinimized()
                setVisibility('minimized')
              }}
              className={cn(
                'p-2 min-h-11 min-w-11 flex items-center justify-center rounded cursor-pointer',
                animation.button.pronounced,
                bg.hover.default,
                text.tertiary
              )}
              title="Minimize player"
              aria-label="Minimize player"
            >
              <ChevronDown size={18} className="transition-transform hover:translate-y-0.5" />
            </button>
          </div>
        </div>

        <style>{`
          .slider::-webkit-slider-thumb {
            appearance: none;
            width: 14px;
            height: 14px;
            border-radius: 50%;
            background: white;
            cursor: pointer;
            box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.3), 0 2px 8px rgba(0, 0, 0, 0.3);
            transition: box-shadow 0.15s ease;
          }

          .slider::-webkit-slider-thumb:hover {
            box-shadow: 0 0 0 6px rgba(59, 130, 246, 0.4), 0 2px 12px rgba(0, 0, 0, 0.4);
          }

          .slider::-moz-range-thumb {
            width: 14px;
            height: 14px;
            border-radius: 50%;
            background: white;
            cursor: pointer;
            border: none;
            box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.3), 0 2px 8px rgba(0, 0, 0, 0.3);
          }

          .volume-slider::-webkit-slider-thumb {
            appearance: none;
            width: 12px;
            height: 12px;
            border-radius: 50%;
            background: white;
            cursor: pointer;
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.3), 0 1px 4px rgba(0, 0, 0, 0.3);
          }

          .volume-slider::-moz-range-thumb {
            width: 12px;
            height: 12px;
            border-radius: 50%;
            background: white;
            cursor: pointer;
            border: none;
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.3), 0 1px 4px rgba(0, 0, 0, 0.3);
          }
        `}</style>
      </div>

      {/* Queue Panel */}
      <QueuePanel isOpen={showQueue} onClose={() => setShowQueue(false)} />

      {/* Keyboard Shortcuts Help */}
      <KeyboardShortcuts isOpen={showShortcuts} onClose={() => setShowShortcuts(false)} />

      {/* Album Art Modal */}
      {showAlbumArt && currentTrack && (
        <div
          className="fixed inset-0 z-100 flex items-center justify-center bg-black/90 backdrop-blur-md animate-in fade-in duration-200"
          onClick={() => setShowAlbumArt(false)}
        >
          <div className="relative w-[85vw] h-[85vh] max-w-5xl p-8 animate-in zoom-in-95 duration-300">
            <button
              onClick={() => setShowAlbumArt(false)}
              className="absolute -top-2 -right-2 p-3 bg-black/60 rounded-full hover:bg-black/80 hover:scale-110 transition-all duration-200 z-10 shadow-xl cursor-pointer"
              title="Close (Esc)"
              aria-label="Close album art view"
            >
              <svg
                className="w-6 h-6 text-white"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
            <div
              className="w-full h-full rounded-2xl overflow-hidden shadow-2xl ring-1 ring-white/10 animate-in slide-in-from-bottom-4 duration-300"
              onClick={(e) => e.stopPropagation()}
            >
              <MediaPoster
                mediaId={currentTrack.id}
                mediaType="media"
                alt={currentTrack.album || currentTrack.title}
                className="w-full h-full object-contain"
                preset="xlarge"
                aspectRatio="square"
                fallbackIcon="🎵"
              />
            </div>
            <div className="mt-6 text-center animate-in fade-in slide-in-from-bottom-2 duration-500 delay-100">
              <h3 className="text-2xl font-bold text-white drop-shadow-lg">{currentTrack.title}</h3>
              <p className="text-base text-neutral-200 mt-2 drop-shadow-md">
                {currentTrack.artist || 'Unknown Artist'}
                {currentTrack.album && ` • ${currentTrack.album}`}
              </p>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

export type { AudioPlayerProps } from './AudioPlayer.types'
export { AudioPlayer }
