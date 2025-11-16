import { useMediaProgress } from '@/lib/hooks/useProgress'
import { getProgressPercentage } from '@/lib/utils'
import { formatDuration } from '@/lib/utils/format'
import { getCodecBadgeColor } from '@/lib/utils/media'
import { formatResolutionLabel } from '@/lib/utils/quality'
import type { EpisodeCardProps } from './EpisodeCard.types'

const EpisodeCard = ({ episode, onClick }: EpisodeCardProps) => {
  const { data: progress } = useMediaProgress(episode.id, true)

  const handleClick = () => {
    onClick?.()
  }

  const resolution = formatResolutionLabel(episode.height)
  const episodeNumber = `S${episode.season}E${episode.episode}`
  
  // Format air date if available
  const formatAirDate = (dateStr?: string) => {
    if (!dateStr) {
      return null
    }
    try {
      const date = new Date(dateStr)
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
    } catch {
      return null
    }
  }
  
  const airDate = formatAirDate(episode.air_date)

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-xl transition-all hover:scale-105 duration-200"
      onClick={handleClick}
    >
      {/* Thumbnail with badges */}
      <div className="aspect-video bg-linear-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-4xl relative">
        🎬
        {/* Badges overlay */}
        <div className="absolute top-2 left-2 right-2 flex justify-between">
          <div className="flex gap-1">
            <span className="px-2 py-1 text-xs font-semibold bg-indigo-600 text-white rounded">
              {episodeNumber}
            </span>
            {resolution && (
              <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
                {resolution}
              </span>
            )}
          </div>
          {episode.video_codec && (
            <span
              className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(
                episode.video_codec
              )}`}
            >
              {episode.video_codec.toUpperCase()}
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
          <div className="flex-1">
            <div className="text-xs text-gray-500 mb-1">{episodeNumber}</div>
            <h3 className="font-semibold text-sm line-clamp-2">
              {episode.episode_title || episode.title}
            </h3>
          </div>
          {progress?.is_watched && (
            <span className="text-green-500 shrink-0" title="Watched">
              ✓
            </span>
          )}
        </div>
        
        {/* Air Date */}
        {airDate && (
          <p className="text-xs text-gray-500 mb-2">
            Aired: {airDate}
          </p>
        )}
        
        {/* Description/Plot from NFO */}
        {episode.description && (
          <p className="text-xs text-gray-600 line-clamp-2 mb-2">
            {episode.description}
          </p>
        )}
        
        {/* Duration and Progress */}
        <div className="flex items-center justify-between text-xs text-gray-500 mt-2">
          {episode.duration > 0 && <span>{formatDuration(episode.duration)}</span>}
          {progress && getProgressPercentage(progress) > 0 && !progress.is_watched && (
            <span className="text-blue-600 font-medium">
              {Math.floor(getProgressPercentage(progress))}% watched
            </span>
          )}
        </div>
        
        {/* IMDb/TVDb indicators */}
        {(episode.imdb_id || episode.tvdb_id) && (
          <div className="flex gap-1 mt-2">
            {episode.imdb_id && (
              <span className="text-xs text-yellow-600 font-bold" title="IMDb ID available">
                IMDb
              </span>
            )}
            {episode.tvdb_id && (
              <span className="text-xs text-blue-600 font-bold" title="TVDb ID available">
                TVDb
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export type { EpisodeCardProps } from './EpisodeCard.types'
export { EpisodeCard }
