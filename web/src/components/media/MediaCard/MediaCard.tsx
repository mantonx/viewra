import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import { cn } from '@/lib/utils'
import type { MediaCardProps } from './MediaCard.types'

/**
 * MediaCard - Reusable card component for all media types (movies, TV shows, music)
 *
 * Provides consistent card styling and hover behavior across all media types.
 * Supports custom content rendering for media-specific information.
 */
export const MediaCard = ({
  mediaId,
  mediaType,
  imageAlt,
  imageFallback = '🎬',
  aspectRatio = '2/3',
  onClick,
  onPlay,
  badges,
  overlays,
  infoContent,
  title,
  year,
  playIconType = 'play',
}: MediaCardProps) => {
  const [isHovered, setIsHovered] = useState(false)

  const handleCardClick = (_e: React.MouseEvent) => {
    // Only trigger onClick if we're NOT clicking on the play button area
    onClick?.()
  }

  const handlePlayClick = (e: React.MouseEvent) => {
    // Stop all event propagation and default behavior
    e.stopPropagation()
    e.preventDefault()

    // If onPlay is provided (TV shows), trigger it
    // Otherwise fall back to onClick (movies)
    if (onPlay) {
      onPlay()
    } else {
      onClick?.()
    }
  }

  const aspectClass = aspectRatio === 'square' ? 'aspect-square' : 'aspect-2/3'

  return (
    <div
      className={cn(
        'rounded-xl overflow-hidden cursor-pointer relative',
        'bg-white dark:bg-neutral-900',
        'shadow-sm dark:shadow-none',
        'border border-neutral-200/50 dark:border-white/5',
        'transition-all duration-200 ease-out',
        // Hover effects - refined lift with glow
        'hover:scale-[1.03] hover:z-50',
        'hover:shadow-xl hover:shadow-neutral-900/10',
        'hover:border-neutral-300/50',
        'dark:hover:shadow-2xl dark:hover:shadow-primary-500/10',
        'dark:hover:border-primary-500/20'
      )}
      onClick={handleCardClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Thumbnail */}
      <div className={`${aspectClass} relative`}>
        <MediaPoster
          mediaId={mediaId}
          mediaType={mediaType}
          alt={imageAlt}
          className="w-full h-full absolute inset-0"
          preset="medium"
          fallbackIcon={imageFallback}
        />

        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div
            className="pointer-events-auto"
            onClick={handlePlayClick}
          >
            <HoverPlayButton isParentHovered={isHovered} iconType={playIconType} size="large" />
          </div>
        </div>

        {/* Badge overlays */}
        {badges && (
          <div className="absolute top-2 left-2 right-2 flex justify-between items-start z-10">
            {badges}
          </div>
        )}

        {/* Positioned overlays (progress bar, watched badge, etc.) */}
        {overlays}
      </div>

      {/* Info section */}
      {infoContent ? (
        <div className="p-3">{infoContent}</div>
      ) : (title || year) ? (
        <div className="p-3">
          {title && (
            <h3 className="font-medium text-sm text-neutral-900 dark:text-white line-clamp-1">
              {title}
            </h3>
          )}
          {year && year > 0 && (
            <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
              {year}
            </p>
          )}
        </div>
      ) : null}
    </div>
  )
}
