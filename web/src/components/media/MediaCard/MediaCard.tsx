import { useMediaProgress } from '@/lib/hooks/useProgress'
import { getProgressPercentage } from '@/lib/utils'
import { formatResolutionLabel } from '@/lib/utils/quality'
import { getCodecBadgeColor } from '@/lib/utils/media'
import { MediaPoster } from '@/components/media/MediaPoster'
import { ProgressBar } from '@/components/media/ProgressBar'
import { WatchedBadge } from '@/components/media/WatchedBadge'
import type { MediaCardProps } from './MediaCard.types'

const MediaCard = ({ media, onClick }: MediaCardProps) => {
  const { data: progress } = useMediaProgress(media.id, true)

  const handleClick = () => {
    onClick?.()
  }

  const resolution = formatResolutionLabel(media.height)

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
      onClick={handleClick}
    >
      {/* Thumbnail with badges */}
      <div className="aspect-2/3 relative">
        {media.id ? (
          <MediaPoster
            mediaId={media.id}
            alt={media.title || 'Media'}
            className="w-full h-full absolute inset-0"
          />
        ) : (
          <div className="bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-4xl w-full h-full">
            🎬
          </div>
        )}
        {/* Badges overlay */}
        <div className="absolute top-2 left-2 right-2 flex justify-between">
          <div className="flex gap-1">
            {media.is_extra && (
              <span className="px-2 py-1 text-xs font-semibold bg-yellow-500 text-black rounded">
                EXTRA
              </span>
            )}
            {resolution && (
              <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
                {resolution}
              </span>
            )}
          </div>
          {media.video_codec && (
            <span
              className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(
                media.video_codec
              )}`}
            >
              {media.video_codec.toUpperCase()}
            </span>
          )}
        </div>
        <ProgressBar progress={progress} />
        <WatchedBadge isWatched={progress?.is_watched} />
      </div>

      {/* Info */}
      <div className="p-3">
        <div className="flex items-start justify-between gap-2 mb-1">
          <h3 className="font-semibold text-sm line-clamp-2 flex-1">{media.title}</h3>
          {progress?.is_watched && (
            <span className="text-green-500 shrink-0" title="Watched">
              ✓
            </span>
          )}
        </div>
        <div className="flex items-center justify-between text-xs text-gray-500">
          {media.file_size && <span>{(media.file_size / 1024 / 1024 / 1024).toFixed(2)} GB</span>}
          {progress && getProgressPercentage(progress) > 0 && !progress.is_watched && (
            <span className="text-blue-600 font-medium">
              {Math.floor(getProgressPercentage(progress))}% watched
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

export type { MediaCardProps } from './MediaCard.types'
export { MediaCard }
