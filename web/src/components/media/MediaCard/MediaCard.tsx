import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import { bg, shadow } from '@/styles/semantic'
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
        bg.elevated,
        shadow.default,
        'rounded-lg overflow-hidden cursor-pointer transition-all duration-200 relative',
        'hover:shadow-xl hover:scale-105 hover:z-50',
        'dark:hover:shadow-neutral-950'
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
      {infoContent && <div className="p-3">{infoContent}</div>}
    </div>
  )
}
