import type { TrackListItemProps } from './TrackListItem.types'

const formatDuration = (seconds: number): string => {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const TrackListItem = ({ track, isPlaying, onClick }: TrackListItemProps) => {
  const handleClick = () => {
    onClick?.()
  }

  return (
    <div
      className={`flex items-center gap-4 p-3 rounded-lg cursor-pointer transition-all ${
        isPlaying
          ? 'bg-rose-50 hover:bg-rose-100'
          : 'hover:bg-gray-50 hover:scale-[1.01] hover:shadow-md'
      }`}
      onClick={handleClick}
    >
      {/* Track number / playing indicator */}
      <div className="w-8 text-center shrink-0">
        {isPlaying ? (
          <span className="text-rose-600 font-semibold">▶</span>
        ) : (
          <span className="text-gray-500 text-sm">{track.track_number || '-'}</span>
        )}
      </div>

      {/* Track info */}
      <div className="flex-1 min-w-0">
        <h4 className={`text-sm font-medium truncate ${isPlaying ? 'text-rose-600' : 'text-gray-900'}`}>
          {track.title}
        </h4>
        <div className="flex items-center gap-2 mt-1">
          {track.artist && (
            <p className="text-xs text-gray-600 truncate">{track.artist}</p>
          )}
          {/* Metadata badges */}
          <div className="flex items-center gap-1.5">
            {track.year && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 text-blue-700 rounded">
                {track.year}
              </span>
            )}
            {track.genre && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-purple-100 text-purple-700 rounded truncate max-w-[100px]">
                {track.genre}
              </span>
            )}
            {track.bitrate && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-gray-100 text-gray-700 rounded">
                {track.bitrate} kbps
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Disc number (if multi-disc) */}
      {track.disc_number && track.disc_number > 1 && (
        <div className="shrink-0">
          <span className="px-2 py-1 text-xs bg-gray-200 text-gray-700 rounded">
            Disc {track.disc_number}
          </span>
        </div>
      )}

      {/* Duration */}
      <div className="text-sm text-gray-500 shrink-0 w-12 text-right">
        {formatDuration(track.duration)}
      </div>
    </div>
  )
}

export type { TrackListItemProps } from './TrackListItem.types'
export { TrackListItem }
