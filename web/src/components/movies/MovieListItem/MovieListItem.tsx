import { MediaListItem } from '@/components/media'
import { bg, text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { MovieListItemProps } from './MovieListItem.types'

/**
 * MovieListItem - List view representation of a movie
 * Shows poster thumbnail, title, year, and plot summary in a horizontal layout
 */
export const MovieListItem = ({ movie, onClick }: MovieListItemProps) => {
  // Determine if movie is newly added (within last 7 days)
  const isNew = Boolean(
    movie.created_at &&
    Date.now() - new Date(movie.created_at).getTime() < 7 * 24 * 60 * 60 * 1000
  )

  return (
    <MediaListItem
      mediaId={movie.id}
      mediaType="movie"
      title={movie.title}
      imageAlt={`${movie.title} poster`}
      imageFallback="🎬"
      aspectRatio="2/3"
      iconType="play"
      showNewBadge={isNew}
      onClick={onClick}
      ariaLabel={`Play ${movie.title}`}
    >
      <div className="flex items-start justify-between gap-2 mb-1">
        <h3 className={cn('text-lg font-semibold truncate', text.primary)}>
          {movie.title}
        </h3>
        {movie.year && (
          <span className={cn('shrink-0 text-sm font-medium', text.secondary)}>
            {movie.year}
          </span>
        )}
      </div>

      {/* Metadata */}
      <div className={cn('flex flex-wrap gap-3 mb-2 text-sm', text.secondary)}>
        {movie.runtime_minutes && (
          <span>{Math.floor(movie.runtime_minutes)} min</span>
        )}
        {movie.content_rating && (
          <span className={cn('px-2 py-0.5 rounded text-xs font-medium', bg.tertiary)}>
            {movie.content_rating}
          </span>
        )}
      </div>

      {/* Genres */}
      {movie.genre && movie.genre.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {movie.genre.slice(0, 3).map((genre) => (
            <span
              key={genre}
              className={cn('px-2 py-0.5 text-xs rounded', bg.tertiary, text.secondary)}
            >
              {genre}
            </span>
          ))}
        </div>
      )}

      {/* Plot Summary */}
      {movie.plot && (
        <p className={cn('text-sm line-clamp-2 leading-relaxed', text.secondary)}>
          {movie.plot}
        </p>
      )}

      {/* Director/Cast */}
      {(movie.director || movie.cast) && (
        <div className={cn('mt-2 text-xs', text.tertiary)}>
          {movie.director && <span>Directed by {movie.director}</span>}
          {movie.director && movie.cast && movie.cast.length > 0 && (
            <span> • </span>
          )}
          {movie.cast && movie.cast.length > 0 && (
            <span>Starring {movie.cast.slice(0, 2).join(', ')}</span>
          )}
        </div>
      )}
    </MediaListItem>
  )
}
