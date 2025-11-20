import { MediaCard } from '@/components/media/MediaCard'
import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick }: TVShowCardProps) => {
  return (
    <MediaCard
      mediaId={show.id}
      mediaType="tv-show"
      imageAlt={show.title}
      imageFallback="📺"
      aspectRatio="2/3"
      onClick={onClick}
      playIconType="play"
      badges={
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          TV SHOW
        </span>
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-2">{show.title}</h3>
          <div className="flex items-center justify-between text-xs text-gray-600">
            <span>
              {show.season_count} {show.season_count === 1 ? 'Season' : 'Seasons'}
            </span>
            <span>
              {show.episode_count} {show.episode_count === 1 ? 'Episode' : 'Episodes'}
            </span>
          </div>
        </>
      }
    />
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
