import { useMediaProgress } from '@/lib/hooks/useProgress'
import { getProgressPercentage } from '@/lib/utils'
import { formatResolutionLabel } from '@/lib/utils/quality'
import { getCodecBadgeColor } from '@/lib/utils/media'
import { MediaPoster } from '@/components/media/MediaPoster'
import type { MovieCardProps} from './MovieCard.types'

const MovieCard = ({ movie, onClick }: MovieCardProps) => {
  const { data: progress } = useMediaProgress(movie.id, true)

  const handleClick = () => {
    onClick?.()
  }

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

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-xl transition-all hover:scale-105 duration-200"
      onClick={handleClick}
    >
      {/* Thumbnail with badges */}
      <div className="aspect-2/3 relative">
        <MediaPoster
          mediaId={movie.id}
          alt={movie.title}
          className="w-full h-full absolute inset-0"
          preset="medium"
        />
        {/* Top badges */}
        <div className="absolute top-2 left-2 right-2 flex justify-between">
          <div className="flex gap-1 flex-wrap">
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
              <span className="px-2 py-1 text-xs font-semibold bg-gray-800 bg-opacity-90 text-white rounded border border-gray-600">
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
        </div>
        
        {/* Progress bar - overlaid at bottom of thumbnail */}
        {progress && getProgressPercentage(progress) > 0 && (
          <div className="absolute bottom-0 left-0 right-0 h-1 bg-black bg-opacity-30">
            <div
              className={`h-full transition-all ${
                progress.is_watched ? 'bg-green-500' : 'bg-blue-500'
              }`}
              style={{ width: `${Math.min(getProgressPercentage(progress), 100)}%` }}
            />
          </div>
        )}
        
        {/* Watched badge overlay */}
        {progress?.is_watched && (
          <div className="absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2">
            <div className="bg-green-500 text-white px-3 py-2 rounded-full font-semibold text-sm shadow-lg flex items-center gap-1">
              <span>✓</span>
              <span>Watched</span>
            </div>
          </div>
        )}
      </div>

      {/* Info */}
      <div className="p-3">
        <div className="flex items-start justify-between gap-2 mb-1">
          <h3 className="font-semibold text-sm line-clamp-2 flex-1">{movie.title}</h3>
          {progress?.is_watched && (
            <span className="text-green-500 shrink-0" title="Watched">
              ✓
            </span>
          )}
        </div>
        
        {/* Year and Duration */}
        <div className="flex items-center gap-2 text-xs text-gray-600 mb-2">
          {movie.year && <span className="font-medium">{movie.year}</span>}
          {movie.year && duration && <span>•</span>}
          {duration && <span>{duration}</span>}
        </div>
        
        {/* Genre */}
        {primaryGenre && (
          <div className="mb-2">
            <span className="inline-block px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded">
              {primaryGenre}
            </span>
          </div>
        )}
        
        {/* Plot preview - show first 80 characters */}
        {movie.plot && (
          <p className="text-xs text-gray-600 line-clamp-2 mb-2">
            {movie.plot.substring(0, 100)}
            {movie.plot.length > 100 ? '...' : ''}
          </p>
        )}
        
        {/* Director */}
        {movie.director && (
          <p className="text-xs text-gray-500 truncate">
            <span className="font-medium">Dir:</span> {movie.director}
          </p>
        )}
        
        {/* Progress or file size */}
        <div className="flex items-center justify-between text-xs text-gray-500 mt-2">
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
                <span className="text-yellow-600 font-bold" title="IMDb ID available">
                  IMDb
                </span>
              )}
              {movie.tmdb_id && (
                <span className="text-blue-600 font-bold" title="TMDb ID available">
                  TMDb
                </span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export type { MovieCardProps } from './MovieCard.types'
export { MovieCard }
