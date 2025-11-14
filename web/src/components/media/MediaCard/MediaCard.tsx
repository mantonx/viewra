import type { MediaCardProps } from './MediaCard.types'
import { useMediaProgress } from '../../../lib/hooks/useProgress'
import { getProgressPercentage } from '../../../lib/utils'

const MediaCard = ({ media, onClick }: MediaCardProps) => {
  const { data: progress } = useMediaProgress(media.id, true)

  const handleClick = () => {
    onClick?.()
  }

  // Get resolution label
  const getResolutionLabel = () => {
    if (!media.width || !media.height) return null
    if (media.height >= 2160) return '4K'
    if (media.height >= 1080) return '1080p'
    if (media.height >= 720) return '720p'
    return `${media.height}p`
  }

  // Get codec badge color
  const getCodecBadgeColor = (codec?: string) => {
    if (!codec) return 'bg-gray-500'
    const c = codec.toLowerCase()
    if (c.includes('hevc') || c.includes('h265')) return 'bg-green-600'
    if (c.includes('h264') || c.includes('avc')) return 'bg-blue-600'
    return 'bg-purple-600'
  }

  const resolution = getResolutionLabel()

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
      onClick={handleClick}
    >
      {/* Thumbnail with badges */}
      <div className="aspect-[2/3] bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-4xl relative">
        🎬

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
            <span className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(media.video_codec)}`}>
              {media.video_codec.toUpperCase()}
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
          <h3 className="font-semibold text-sm line-clamp-2 flex-1">{media.title}</h3>
          {progress?.is_watched && (
            <span className="text-green-500 flex-shrink-0" title="Watched">
              ✓
            </span>
          )}
        </div>
        <div className="flex items-center justify-between text-xs text-gray-500">
          {media.file_size && (
            <span>{(media.file_size / 1024 / 1024 / 1024).toFixed(2)} GB</span>
          )}
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

export { MediaCard }
export type { MediaCardProps } from './MediaCard.types'
