import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import { bg, shadow, ring } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { MediaListItemProps } from './MediaListItem.types'

/**
 * MediaListItem - Unified list view component for all media types
 * Provides consistent horizontal layout with thumbnail and content
 */
export const MediaListItem = ({
  mediaId,
  mediaType,
  title,
  imageAlt,
  imageFallback = '🎬',
  aspectRatio = '2/3',
  iconType = 'play',
  rounded = 'default',
  showNewBadge = false,
  children,
  onClick,
  ariaLabel,
}: MediaListItemProps) => {
  const [isHovered, setIsHovered] = useState(false)

  const handleClick = () => {
    if (onClick) {
      onClick()
    }
  }

  // Determine thumbnail dimensions based on aspect ratio
  const thumbnailClass = aspectRatio === 'square' ? 'w-24 h-24' : 'w-24 h-36'
  const thumbnailRounded = rounded === 'full' ? 'rounded-full' : 'rounded'

  return (
    <div
      className={cn(
        'group flex gap-4 p-4 rounded-lg border-2 border-transparent transition-all duration-200 cursor-pointer',
        bg.elevated,
        shadow.default,
        'hover:shadow-lg hover:border-neutral-300 dark:hover:border-neutral-700',
        ring.focus
      )}
      onClick={handleClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          handleClick()
        }
      }}
      aria-label={ariaLabel || `View ${title}`}
    >
      {/* Thumbnail with Play/View Button */}
      <div className={cn('shrink-0 relative', thumbnailClass)}>
        <MediaPoster
          mediaType={mediaType}
          mediaId={mediaId}
          alt={imageAlt}
          className={cn('w-full h-full shadow-sm transition-all duration-200', thumbnailRounded)}
          preset="thumb"
          fallbackIcon={imageFallback}
        />

        {/* Hover Button - centered for non-circular, positioned for circular */}
        {rounded === 'full' ? (
          <HoverPlayButton isParentHovered={isHovered} iconType={iconType} size="small" rounded="full" />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center">
            <HoverPlayButton isParentHovered={isHovered} iconType={iconType} size="small" />
          </div>
        )}

        {/* NEW Badge */}
        {showNewBadge && (
          <span className="absolute top-2 left-2 px-2 py-1 text-xs font-bold bg-linear-to-r from-green-500 to-emerald-600 text-white rounded shadow-lg z-10">
            NEW
          </span>
        )}
      </div>

      {/* Content Area */}
      <div className="flex-1 min-w-0">
        {children}
      </div>
    </div>
  )
}
