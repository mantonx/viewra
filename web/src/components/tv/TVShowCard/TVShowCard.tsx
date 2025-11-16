import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick }: TVShowCardProps) => {
  const handleClick = () => {
    onClick?.()
  }

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
      onClick={handleClick}
    >
      {/* Thumbnail with show icon */}
      <div className="aspect-2/3 bg-linear-to-br from-indigo-600 to-purple-700 flex items-center justify-center text-white text-5xl relative">
        📺
        {/* Badge overlays */}
        <div className="absolute top-2 left-2 right-2 flex justify-between">
          <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
            TV SHOW
          </span>
        </div>
      </div>

      {/* Info */}
      <div className="p-3">
        <h3 className="font-semibold text-sm line-clamp-2 mb-2">{show.title}</h3>
        <div className="flex items-center justify-between text-xs text-gray-600">
          <span>
            {show.season_count} {show.season_count === 1 ? 'Season' : 'Seasons'}
          </span>
          <span>
            {show.episode_count} {show.episode_count === 1 ? 'Episode' : 'Episodes'}
          </span>
        </div>
      </div>
    </div>
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
