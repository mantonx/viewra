import { getProgressPercentage, cn } from '@/lib/utils'
import { text, bg } from '@/styles/semantic'
import { RatingButtons } from '@/components/media/RatingButtons'
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
  rating,
}: MediaMetadataProps) => {
  // Format duration to hours and minutes
  const formatDuration = (seconds?: number) => {
    if (!seconds) {return null}
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
        <h3 className={cn('font-semibold text-sm line-clamp-2 flex-1', text.primary)}>
          {title}
        </h3>
        {progress?.is_watched && getProgressPercentage(progress) >= 95 && (
          <span className="text-green-500 shrink-0" title="Watched">
            ✓
          </span>
        )}
      </div>

      {/* Rating buttons */}
      {rating && (
        <div className="mb-2" onClick={(e) => e.stopPropagation()}>
          <RatingButtons
            entityType={rating.entityType}
            entityId={rating.entityId}
            size="sm"
          />
        </div>
      )}

      {/* Year and Duration OR Season/Episode counts */}
      <div className={cn('flex items-center gap-2 text-xs mb-2', text.secondary)}>
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
          <span className={cn('inline-block px-2 py-1 text-xs rounded', bg.tertiary, text.secondary)}>
            {primaryGenre}
          </span>
        </div>
      )}

      {/* Plot preview */}
      {plot && (
        <p
          className={cn('text-xs line-clamp-2 mb-2', text.secondary)}
          title={plot.length > 100 ? plot : undefined}
        >
          {plot.substring(0, 100)}
          {plot.length > 100 ? '...' : ''}
        </p>
      )}

      {/* Director */}
      {director && (
        <p className={cn('text-xs truncate', text.tertiary)}>
          <span className="font-medium">Dir:</span> {director}
        </p>
      )}

      {/* Progress or file size */}
      <div className={cn('flex items-center justify-between text-xs mt-2', text.tertiary)}>
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
