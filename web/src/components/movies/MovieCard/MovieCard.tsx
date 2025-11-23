import { useBatchProgress } from '@/lib/hooks'
import { getProgressPercentage } from '@/lib/utils'
import { formatResolutionLabel } from '@/lib/utils/quality'
import { getCodecBadgeColor } from '@/lib/utils/media'
import { MediaCard } from '@/components/media/MediaCard'
import { ProgressBar } from '@/components/media/ProgressBar'
import type { MovieCardProps} from './MovieCard.types'

const MovieCard = ({ movie, onClick }: MovieCardProps) => {
  const { progress } = useBatchProgress(movie.id)

  const resolution = formatResolutionLabel(movie.height)
  
  // Format duration to hours and minutes
  const formatDuration = (seconds?: number) => {
    if (!seconds) {
      return null
    }
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    if (hours > 0) {
      return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
  }

  const duration = formatDuration(movie.duration)

  // Get first genre if multiple
  const primaryGenre = movie.genre && movie.genre.length > 0 ? movie.genre[0] : null

  // Check if movie is newly added (within last 7 days)
  const isNew = movie.created_at &&
    (Date.now() - new Date(movie.created_at).getTime()) < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={movie.id}
      mediaType="movie"
      imageAlt={movie.title}
      aspectRatio="2/3"
      onClick={onClick}
      playIconType="play"
      badges={
        <>
          <div className="flex gap-1 flex-wrap">
            {isNew && (
              <span className="px-2 py-1 text-xs font-bold bg-linear-to-r from-green-500 to-emerald-600 text-white rounded shadow-lg">
                NEW
              </span>
            )}
            {movie.is_extra && (
              <span className="px-2 py-1 text-xs font-semibold bg-yellow-500 text-black rounded">
                EXTRA
              </span>
            )}
            {resolution && (
              <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
                {resolution}
              </span>
            )}
            {movie.content_rating && (
              <span className="px-2 py-1 text-xs font-semibold bg-neutral-800 dark:bg-neutral-700 bg-opacity-90 text-white rounded border border-neutral-600 dark:border-neutral-500">
                {movie.content_rating}
              </span>
            )}
          </div>
          {movie.video_codec && (
            <span
              className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(
                movie.video_codec
              )}`}
            >
              {movie.video_codec.toUpperCase()}
            </span>
          )}
        </>
      }
      overlays={
        <>
          <ProgressBar progress={progress} />
        </>
      }
      infoContent={
        <>
          <div className="flex items-start justify-between gap-2 mb-1">
            <h3 className="font-semibold text-sm line-clamp-2 flex-1">{movie.title}</h3>
            {progress?.is_watched && getProgressPercentage(progress) >= 95 && (
              <span className="text-green-500 shrink-0" title="Watched">
                ✓
              </span>
            )}
          </div>

          {/* Year and Duration */}
          <div className="flex items-center gap-2 text-xs text-neutral-600 dark:text-neutral-400 mb-2">
            {movie.year && <span className="font-medium">{movie.year}</span>}
            {movie.year && duration && <span>•</span>}
            {duration && <span>{duration}</span>}
          </div>

          {/* Genre */}
          {primaryGenre && (
            <div className="mb-2">
              <span className="inline-block px-2 py-1 text-xs bg-neutral-100 dark:bg-neutral-800 text-neutral-700 dark:text-neutral-300 rounded">
                {primaryGenre}
              </span>
            </div>
          )}

          {/* Plot preview - show first 80 characters */}
          {movie.plot && (
            <p className="text-xs text-neutral-600 dark:text-neutral-400 line-clamp-2 mb-2">
              {movie.plot.substring(0, 100)}
              {movie.plot.length > 100 ? '...' : ''}
            </p>
          )}

          {/* Director */}
          {movie.director && (
            <p className="text-xs text-neutral-500 dark:text-neutral-500 truncate">
              <span className="font-medium">Dir:</span> {movie.director}
            </p>
          )}

          {/* Progress or file size */}
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-500 mt-2">
            {progress && getProgressPercentage(progress) > 0 && !progress.is_watched ? (
              <span className="text-blue-600 font-medium">
                {Math.floor(getProgressPercentage(progress))}% watched
              </span>
            ) : movie.file_size ? (
              <span>{(movie.file_size / 1024 / 1024 / 1024).toFixed(1)} GB</span>
            ) : (
              <span></span>
            )}

            {/* IMDb/TMDb indicators */}
            {(movie.imdb_id || movie.tmdb_id) && (
              <div className="flex gap-1">
                {movie.imdb_id && (
                  <a
                    href={`https://www.imdb.com/title/${movie.imdb_id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-yellow-600 font-bold hover:underline"
                    title="View on IMDb"
                    onClick={(e) => e.stopPropagation()}
                  >
                    IMDb
                  </a>
                )}
                {movie.tmdb_id && (
                  <a
                    href={`https://www.themoviedb.org/movie/${movie.tmdb_id}`}
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
      }
    />
  )
}

export type { MovieCardProps } from './MovieCard.types'
export { MovieCard }
