import { useState } from 'react'
import { cn } from '@/lib/utils'
import { HoverPlayButton } from '@/components/common'
import { useMediaImages, useTVShowImages } from '@/lib/hooks/useMediaImages'
import { getFanartImage, getImageUrl } from '@/lib/types/images'
import type { ContinueWatchingItem } from './widget.types'

interface ContinueWatchingCardProps {
  item: ContinueWatchingItem
  onClick: () => void
  onPlay?: () => void
}

/**
 * ContinueWatchingCard - Horizontal 16:9 card for continue watching items
 *
 * Features:
 * - Horizontal aspect ratio (16:9) with backdrop image
 * - Progress bar at the bottom
 * - Remaining time text
 * - Episode badge for TV shows (S2 E4)
 * - Play button overlay on hover
 */
export const ContinueWatchingCard = ({ item, onClick, onPlay }: ContinueWatchingCardProps) => {
  const [isHovered, setIsHovered] = useState(false)

  // Fetch backdrop image based on entity type
  const isMovie = item.entity_type === 'movie'
  const movieQuery = useMediaImages(isMovie ? item.entity_id : undefined)
  const showQuery = useTVShowImages(!isMovie ? item.entity_id : undefined)

  const images = isMovie ? movieQuery.data?.images : showQuery.data?.images
  const fanartImage = images ? getFanartImage(images) : null
  const backdropUrl = fanartImage ? getImageUrl(fanartImage.id, 'large') : null

  const handleCardClick = () => {
    onClick()
  }

  const handlePlayClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    if (onPlay) {
      onPlay()
    } else {
      onClick()
    }
  }

  // Format episode badge text
  const episodeBadge = item.episode_context
    ? `S${item.episode_context.season} E${item.episode_context.episode}`
    : null

  return (
    <div
      className={cn(
        'w-56 shrink-0 cursor-pointer',
        'transition-transform duration-200 ease-out',
        'hover:scale-[1.02]'
      )}
      onClick={handleCardClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Card Container */}
      <div
        className={cn(
          'relative aspect-video rounded-xl overflow-hidden',
          'bg-neutral-200 dark:bg-neutral-800',
          'shadow-sm dark:shadow-none',
          'border border-neutral-200/50 dark:border-white/5',
          'transition-all duration-200',
          'hover:shadow-xl hover:shadow-neutral-900/10',
          'dark:hover:shadow-2xl dark:hover:shadow-primary-500/10',
          'dark:hover:border-primary-500/20'
        )}
      >
        {/* Backdrop Image */}
        {backdropUrl ? (
          <img
            src={backdropUrl}
            alt={item.title}
            className="absolute inset-0 w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center text-4xl text-neutral-400">
            {isMovie ? '🎬' : '📺'}
          </div>
        )}

        {/* Gradient overlay for text readability */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent" />

        {/* Play button overlay */}
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="pointer-events-auto" onClick={handlePlayClick}>
            <HoverPlayButton isParentHovered={isHovered} iconType="play" size="large" />
          </div>
        </div>

        {/* Episode badge for TV shows */}
        {episodeBadge && (
          <div className="absolute top-2 left-2">
            <span
              className={cn(
                'px-2 py-1 text-xs font-medium rounded-md',
                'bg-black/70 text-white backdrop-blur-sm'
              )}
            >
              {episodeBadge}
            </span>
          </div>
        )}

        {/* Progress bar */}
        {item.progress && item.progress.percent > 0 && (
          <div className="absolute bottom-0 left-0 right-0 h-1 bg-neutral-700/80">
            <div
              className="h-full bg-primary-500 transition-all duration-300"
              style={{ width: `${item.progress.percent}%` }}
            />
          </div>
        )}
      </div>

      {/* Info section */}
      <div className="mt-2 px-0.5">
        <h3 className="font-medium text-sm text-neutral-900 dark:text-white line-clamp-1">
          {item.title}
        </h3>
        <div className="flex items-center gap-2 mt-0.5">
          {item.progress?.remaining_text && (
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              {item.progress.remaining_text}
            </p>
          )}
          {item.episode_context?.episode_title && (
            <p className="text-xs text-neutral-500 dark:text-neutral-400 line-clamp-1">
              {item.episode_context.episode_title}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
