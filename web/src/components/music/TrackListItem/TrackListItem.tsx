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
          ? 'bg-rose-50 dark:bg-rose-950/30 hover:bg-rose-100 dark:hover:bg-rose-950/50'
          : 'hover:bg-neutral-50 dark:hover:bg-neutral-900 hover:scale-[1.01] hover:shadow-md dark:hover:shadow-neutral-950/50'
      }`}
      onClick={handleClick}
    >
      {/* Track number / playing indicator */}
      <div className="w-8 text-center shrink-0">
        {isPlaying ? (
          <span className="text-rose-600 dark:text-rose-500 font-semibold">▶</span>
        ) : (
          <span className="text-neutral-500 dark:text-neutral-500 text-sm">{track.track_number || '-'}</span>
        )}
      </div>

      {/* Track info */}
      <div className="flex-1 min-w-0">
        <h4 className={`text-sm font-medium truncate ${isPlaying ? 'text-rose-600 dark:text-rose-500' : 'text-neutral-900 dark:text-neutral-50'}`}>
          {track.title}
        </h4>
        <div className="flex items-center gap-2 mt-1">
          {track.artist && (
            <p className="text-xs text-neutral-600 dark:text-neutral-400 truncate">{track.artist}</p>
          )}
          {/* Metadata badges */}
          <div className="flex items-center gap-1.5">
            {track.year && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                {track.year}
              </span>
            )}
            {track.genre && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-purple-100 dark:bg-purple-900 text-purple-700 dark:text-purple-300 rounded truncate max-w-[100px]">
                {track.genre}
              </span>
            )}
            {track.bitrate && (
              <span className="px-1.5 py-0.5 text-[10px] font-medium bg-neutral-100 dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300 rounded">
                {track.bitrate} kbps
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Disc number (if multi-disc) */}
      {track.disc_number && track.disc_number > 1 && (
        <div className="shrink-0">
          <span className="px-2 py-1 text-xs bg-neutral-200 dark:bg-neutral-800 text-neutral-700 dark:text-neutral-300 rounded">
            Disc {track.disc_number}
          </span>
        </div>
      )}

      {/* Duration */}
      <div className="text-sm text-neutral-500 dark:text-neutral-500 shrink-0 w-12 text-right">
        {formatDuration(track.duration)}
      </div>
    </div>
  )
}

export type { TrackListItemProps } from './TrackListItem.types'
export { TrackListItem }
