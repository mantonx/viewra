import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
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
  badges,
  infoContent,
  playIconType = 'play',
}: MediaCardProps) => {
  const [isHovered, setIsHovered] = useState(false)

  const handleClick = () => {
    onClick?.()
  }

  const aspectClass = aspectRatio === 'square' ? 'aspect-square' : 'aspect-2/3'

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-xl transition-all hover:scale-105 duration-200"
      onClick={handleClick}
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

        <HoverPlayButton isParentHovered={isHovered} iconType={playIconType} size="large" />

        {/* Badge overlays */}
        {badges && (
          <div className="absolute top-2 left-2 right-2 flex justify-between items-start z-10">
            {badges}
          </div>
        )}
      </div>

      {/* Info section */}
      {infoContent && <div className="p-3">{infoContent}</div>}
    </div>
  )
}
