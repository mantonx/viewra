import { MediaCard } from '@/components/media/MediaCard'
import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick, onPlay }: TVShowCardProps) => {
  return (
    <MediaCard
      mediaId={show.id ?? 0}
      mediaType="tv-show"
      imageAlt={show.title ?? 'TV Show'}
      imageFallback="📺"
      aspectRatio="2/3"
      onClick={onClick}
      onPlay={onPlay}
      playIconType="play"
      badges={
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          TV SHOW
        </span>
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-2">{show.title ?? 'Unknown Show'}</h3>
          <div className="flex items-center justify-between text-xs text-neutral-600 dark:text-neutral-400">
            <span>
              {show.season_count ?? 0} {show.season_count === 1 ? 'Season' : 'Seasons'}
            </span>
            <span>
              {show.episode_count ?? 0} {show.episode_count === 1 ? 'Episode' : 'Episodes'}
            </span>
          </div>
        </>
      }
    />
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
