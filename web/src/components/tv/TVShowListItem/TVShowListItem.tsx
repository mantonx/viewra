import { MediaListItem } from '@/components/media'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { TVShowListItemProps } from './TVShowListItem.types'

/**
 * TVShowListItem - List view representation of a TV show
 * Shows poster thumbnail, title, and episode/season counts in a horizontal layout
 */
export const TVShowListItem = ({ show, onClick }: TVShowListItemProps) => {
  return (
    <MediaListItem
      mediaId={show.id ?? 0}
      mediaType="tv-show"
      title={show.title ?? 'TV Show'}
      imageAlt={`${show.title ?? 'TV Show'} poster`}
      imageFallback="📺"
      aspectRatio="2/3"
      iconType="view"
      onClick={onClick}
      ariaLabel={`View ${show.title}`}
    >
      <h3 className={cn('text-lg font-semibold mb-1 truncate', text.primary)}>
        {show.title}
      </h3>

      {/* Year and Genre */}
      {(show.year || (show.genre && show.genre.length > 0)) && (
        <div className={cn('flex items-center gap-2 text-sm mb-2', text.secondary)}>
          {show.year && <span className="font-medium">{show.year}</span>}
          {show.year && show.genre && show.genre.length > 0 && <span>•</span>}
          {show.genre && show.genre.length > 0 && <span>{show.genre[0]}</span>}
        </div>
      )}

      {/* Plot */}
      {show.plot && (
        <p className={cn('text-sm line-clamp-2 mb-2', text.secondary)}>
          {show.plot}
        </p>
      )}

      {/* Season/Episode counts and Links */}
      <div className={cn('flex flex-wrap items-center gap-4 text-sm', text.secondary)}>
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

        {/* IMDb/TMDb links */}
        {(show.imdb_id || show.tmdb_id) && (
          <div className="flex gap-2 ml-auto">
            {show.imdb_id && (
              <a
                href={`https://www.imdb.com/title/${show.imdb_id}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-yellow-600 font-bold hover:underline"
                title="View on IMDb"
                onClick={(e) => e.stopPropagation()}
              >
                IMDb
              </a>
            )}
            {show.tmdb_id && (
              <a
                href={`https://www.themoviedb.org/tv/${show.tmdb_id}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 font-bold hover:underline"
                title="View on TMDb"
                onClick={(e) => e.stopPropagation()}
              >
                TMDb
              </a>
            )}
          </div>
        )}
      </div>
    </MediaListItem>
  )
}
