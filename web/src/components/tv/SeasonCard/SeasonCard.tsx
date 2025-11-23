import { MediaPoster } from '@/components/media/MediaPoster'
import type { SeasonCardProps } from './SeasonCard.types'

const SeasonCard = ({ season, showTitle, onClick }: SeasonCardProps) => {
  const handleClick = () => {
    onClick?.()
  }

  const seasonLabel = season.season === 0 ? 'Specials' : `Season ${season.season}`

  return (
    <div
      className="bg-white dark:bg-neutral-900 rounded-lg shadow dark:shadow-neutral-950/50 overflow-hidden cursor-pointer hover:shadow-lg dark:hover:shadow-neutral-950/70 transition-shadow"
      onClick={handleClick}
    >
      {/* Thumbnail */}
      <div className="aspect-[2/3] relative">
        {season.season_id ? (
          <MediaPoster
            mediaId={season.season_id}
            mediaType="tv-season"
            alt={`${showTitle} ${seasonLabel}`}
            className="w-full h-full absolute inset-0"
            preset="medium"
            fallbackIcon="📁"
          />
        ) : (
          <div className="bg-gradient-to-br from-blue-600 to-cyan-600 flex flex-col items-center justify-center text-white h-full">
            <div className="text-6xl mb-2">📁</div>
            <div className="text-xl font-bold">{seasonLabel}</div>
          </div>
        )}
        {/* Episode count badge */}
        <div className="absolute top-2 right-2 z-10">
          <span className="px-3 py-1 text-sm font-semibold bg-black bg-opacity-75 text-white rounded-full">
            {season.episode_count} {season.episode_count === 1 ? 'Episode' : 'Episodes'}
          </span>
        </div>
      </div>

      {/* Info */}
      <div className="p-3">
        <h3 className="font-semibold text-sm line-clamp-1 text-neutral-900 dark:text-neutral-50">{showTitle}</h3>
        <p className="text-xs text-neutral-600 dark:text-neutral-400 mt-1">{seasonLabel}</p>
      </div>
    </div>
  )
}

export type { SeasonCardProps } from './SeasonCard.types'
export { SeasonCard }
