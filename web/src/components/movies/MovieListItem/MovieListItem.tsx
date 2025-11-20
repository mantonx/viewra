import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import type { MovieListItemProps } from './MovieListItem.types'

/**
 * MovieListItem - List view representation of a movie
 * Shows poster thumbnail, title, year, and plot summary in a horizontal layout
 */
export const MovieListItem = ({ movie, onClick }: MovieListItemProps) => {
  const [isHovered, setIsHovered] = useState(false)

  // Determine if movie is newly added (within last 7 days)
  const isNew =
    movie.created_at &&
    Date.now() - new Date(movie.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  const handleClick = () => {
    if (onClick) {
      onClick()
    }
  }

  return (
    <div
      className="group flex gap-4 p-4 bg-white rounded-lg border-2 border-transparent shadow hover:shadow-lg hover:border-rose-500 transition-all duration-200 cursor-pointer focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-2"
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
      aria-label={`Play ${movie.title}`}
    >
      {/* Poster Thumbnail with Centered Play Button */}
      <div className="shrink-0 relative w-24 h-36">
        <MediaPoster
          mediaType="movie"
          mediaId={movie.id}
          alt={`${movie.title} poster`}
          className="w-full h-full rounded shadow-sm transition-all duration-200"
          preset="thumb"
          fallbackIcon="🎬"
        />

        <HoverPlayButton isParentHovered={isHovered} iconType="play" size="small" />

        {isNew && (
          <span className="absolute top-2 left-2 px-2 py-1 text-xs font-bold bg-linear-to-r from-green-500 to-emerald-600 text-white rounded shadow-lg z-10">
            NEW
          </span>
        )}
      </div>

      {/* Movie Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-start justify-between gap-2 mb-1">
          <h3 className="text-lg font-semibold text-gray-900 truncate">
            {movie.title}
          </h3>
          {movie.year && (
            <span className="flex-shrink-0 text-sm text-gray-600 font-medium">
              {movie.year}
            </span>
          )}
        </div>

        {/* Metadata */}
        <div className="flex flex-wrap gap-3 mb-2 text-sm text-gray-600">
          {movie.runtime_minutes && (
            <span>{Math.floor(movie.runtime_minutes)} min</span>
          )}
          {movie.content_rating && (
            <span className="px-2 py-0.5 bg-gray-100 rounded text-xs font-medium">
              {movie.content_rating}
            </span>
          )}
          {movie.imdb_rating && (
            <span className="flex items-center gap-1">
              ⭐ {movie.imdb_rating.toFixed(1)}
            </span>
          )}
        </div>

        {/* Genres */}
        {movie.genre && movie.genre.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-2">
            {movie.genre.slice(0, 3).map((genre) => (
              <span
                key={genre}
                className="px-2 py-0.5 text-xs bg-rose-50 text-rose-700 rounded-full"
              >
                {genre}
              </span>
            ))}
          </div>
        )}

        {/* Plot Summary */}
        {movie.plot && (
          <p className="text-sm text-gray-700 line-clamp-2 leading-relaxed">
            {movie.plot}
          </p>
        )}

        {/* Director/Cast */}
        {(movie.director || movie.cast) && (
          <div className="mt-2 text-xs text-gray-500">
            {movie.director && <span>Directed by {movie.director}</span>}
            {movie.director && movie.cast && movie.cast.length > 0 && (
              <span> • </span>
            )}
            {movie.cast && movie.cast.length > 0 && (
              <span>Starring {movie.cast.slice(0, 2).join(', ')}</span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
