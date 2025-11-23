import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import type { TVShowListItemProps } from './TVShowListItem.types'

/**
 * TVShowListItem - List view representation of a TV show
 * Shows poster thumbnail, title, and episode/season counts in a horizontal layout
 */
export const TVShowListItem = ({ show, onClick }: TVShowListItemProps) => {
  const [isHovered, setIsHovered] = useState(false)

  const handleClick = () => {
    if (onClick) {
      onClick()
    }
  }

  return (
    <div
      className="group flex gap-4 p-4 bg-white dark:bg-neutral-900 rounded-lg border-2 border-transparent shadow dark:shadow-neutral-950/50 hover:shadow-lg dark:hover:shadow-neutral-950/70 hover:border-rose-500 dark:hover:border-rose-500 transition-all duration-200 cursor-pointer focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-2 dark:focus:ring-offset-neutral-900"
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
      aria-label={`View ${show.title}`}
    >
      {/* Poster Thumbnail with Centered View Button */}
      <div className="shrink-0 relative w-24 h-36">
        <MediaPoster
          mediaType="tv-show"
          mediaId={show.id ?? 0}
          alt={`${show.title ?? 'TV Show'} poster`}
          className="w-full h-full rounded shadow-sm transition-all duration-200"
          preset="thumb"
          fallbackIcon="📺"
        />

        <HoverPlayButton isParentHovered={isHovered} iconType="view" size="small" />
      </div>

      {/* Show Info */}
      <div className="flex-1 min-w-0">
        <h3 className="text-lg font-semibold text-neutral-900 dark:text-neutral-50 mb-2 truncate">
          {show.title}
        </h3>

        {/* Metadata */}
        <div className="flex flex-wrap gap-4 text-sm text-neutral-600 dark:text-neutral-400">
          {show.season_count !== undefined && (
            <span className="flex items-center gap-1">
              <span className="font-medium">{show.season_count}</span>
              <span>{show.season_count === 1 ? 'Season' : 'Seasons'}</span>
            </span>
          )}
          {show.episode_count !== undefined && (
            <span className="flex items-center gap-1">
              <span className="font-medium">{show.episode_count}</span>
              <span>{show.episode_count === 1 ? 'Episode' : 'Episodes'}</span>
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
