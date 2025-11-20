import { MediaPoster } from '@/components/media/MediaPoster'
import { ProgressBar } from '@/components/media/ProgressBar'
import { WatchedBadge } from '@/components/media/WatchedBadge'
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
      <div className="aspect-video relative">
        <MediaPoster
          mediaId={episode.id}
          mediaType="tv-episode"
          alt={episode.episode_title || episode.title}
          className="w-full h-full absolute inset-0"
          preset="medium"
          fallbackIcon="🎬"
        />
        {/* Badges overlay */}
        <div className="absolute top-2 left-2 right-2 flex justify-between z-10">
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
        <ProgressBar progress={progress} />
        <WatchedBadge isWatched={progress?.is_watched} />
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
