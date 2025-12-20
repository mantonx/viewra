import { useContext } from 'react'
import { ThemeContext } from '@/contexts/ThemeContext'
import { cn } from '@/lib/utils'
import { text, bg, glassStyles, animation, animationStyles } from '@/styles/semantic'
import { MediaPoster } from '@/components/media/MediaPoster'
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  Loader2,
  ChevronUp,
} from 'lucide-react'
import type { MusicTrackResponse } from '@/lib/types/music'

type AudioPlayerMinimizedProps = {
  currentTrack: MusicTrackResponse
  isPlaying: boolean
  isLoading: boolean
  currentTime: number
  duration: number
  progress: number
  className?: string
  onPlayPause: () => void
  onPrevious: () => void
  onNext: () => void
  onExpand: () => void
}

const formatTime = (seconds: number): string => {
  if (!isFinite(seconds)) {
    return '0:00'
  }
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

export const AudioPlayerMinimized = ({
  currentTrack,
  isPlaying,
  isLoading,
  currentTime,
  duration,
  progress,
  className = '',
  onPlayPause,
  onPrevious,
  onNext,
  onExpand,
}: AudioPlayerMinimizedProps) => {
  const themeContext = useContext(ThemeContext)
  const theme = themeContext?.theme || 'light'

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
              onClick={onPrevious}
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
              onClick={onPlayPause}
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
              onClick={onNext}
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
            onClick={onExpand}
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
