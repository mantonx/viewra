import { getProgressPercentage } from '@/lib/utils'
import type { MediaMetadataProps } from './MediaMetadata.types'

export const MediaMetadata = ({
  title,
  year,
  duration,
  genres,
  plot,
  director,
  fileSize,
  progress,
  links,
  seasonCount,
  episodeCount,
}: MediaMetadataProps) => {
  // Format duration to hours and minutes
  const formatDuration = (seconds?: number) => {
    if (!seconds) return null
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    if (hours > 0) {
      return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
  }

  const formattedDuration = formatDuration(duration)
  const primaryGenre = genres && genres.length > 0 ? genres[0] : null

  return (
    <>
      {/* Title + Watched Indicator */}
      <div className="flex items-start justify-between gap-2 mb-1">
        <h3 className="font-semibold text-sm line-clamp-2 flex-1 text-neutral-900 dark:text-neutral-50">
          {title}
        </h3>
        {progress?.is_watched && getProgressPercentage(progress) >= 95 && (
          <span className="text-green-500 shrink-0" title="Watched">
            ✓
          </span>
        )}
      </div>

      {/* Year and Duration OR Season/Episode counts */}
      <div className="flex items-center gap-2 text-xs text-neutral-600 dark:text-neutral-400 mb-2">
        {year && <span className="font-medium">{year}</span>}
        {year && (formattedDuration || seasonCount !== undefined) && <span>•</span>}
        {formattedDuration && <span>{formattedDuration}</span>}
        {seasonCount !== undefined && (
          <span>
            {seasonCount} {seasonCount === 1 ? 'Season' : 'Seasons'}
          </span>
        )}
        {episodeCount !== undefined && seasonCount !== undefined && <span>•</span>}
        {episodeCount !== undefined && (
          <span>
            {episodeCount} {episodeCount === 1 ? 'Episode' : 'Episodes'}
          </span>
        )}
      </div>

      {/* Genre */}
      {primaryGenre && (
        <div className="mb-2">
          <span className="inline-block px-2 py-1 text-xs bg-neutral-100 dark:bg-neutral-800 text-neutral-700 dark:text-neutral-300 rounded">
            {primaryGenre}
          </span>
        </div>
      )}

      {/* Plot preview */}
      {plot && (
        <p className="text-xs text-neutral-600 dark:text-neutral-400 line-clamp-2 mb-2">
          {plot.substring(0, 100)}
          {plot.length > 100 ? '...' : ''}
        </p>
      )}

      {/* Director */}
      {director && (
        <p className="text-xs text-neutral-500 dark:text-neutral-500 truncate">
          <span className="font-medium">Dir:</span> {director}
        </p>
      )}

      {/* Progress or file size */}
      <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-500 mt-2">
        {progress && getProgressPercentage(progress) > 0 && !progress.is_watched ? (
          <span className="text-blue-600 font-medium">
            {Math.floor(getProgressPercentage(progress))}% watched
          </span>
        ) : fileSize ? (
          <span>{(fileSize / 1024 / 1024 / 1024).toFixed(1)} GB</span>
        ) : (
          <span></span>
        )}

        {/* IMDb/TMDb links */}
        {links && (links.imdb || links.tmdb) && (
          <div className="flex gap-1">
            {links.imdb && (
              <a
                href={`https://www.imdb.com/title/${links.imdb}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-yellow-600 font-bold hover:underline"
                title="View on IMDb"
                onClick={(e) => e.stopPropagation()}
              >
                IMDb
              </a>
            )}
            {links.tmdb && (
              <a
                href={`https://www.themoviedb.org/movie/${links.tmdb}`}
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
    </>
  )
}
