import { TrackListItem } from '../TrackListItem'
import { Button } from '@/components/ui'
import type { TrackListProps } from './TrackList.types'

const TrackList = ({ tracks, currentTrackId, onTrackClick, onPlayAll }: TrackListProps) => {
  // Group tracks by disc number if multi-disc album
  const tracksByDisc = tracks.reduce((acc, track) => {
    const disc = track.disc_number || 1
    if (!acc[disc]) {
      acc[disc] = []
    }
    acc[disc].push(track)
    return acc
  }, {} as Record<number, typeof tracks>)

  const discNumbers = Object.keys(tracksByDisc)
    .map(Number)
    .sort((a, b) => a - b)
  const isMultiDisc = discNumbers.length > 1

  return (
    <div className="space-y-4">
      {/* Play All button */}
      {onPlayAll && tracks.length > 0 && (
        <div className="flex justify-end">
          <Button onClick={onPlayAll} variant="primary">
            <span className="mr-2">▶</span>
            Play All
          </Button>
        </div>
      )}

      {/* Tracks grouped by disc */}
      {discNumbers.map((discNumber) => (
        <div key={discNumber}>
          {isMultiDisc && (
            <h3 className="text-sm font-semibold text-neutral-700 dark:text-neutral-300 mb-2 px-3">
              Disc {discNumber}
            </h3>
          )}
          <div className="space-y-1">
            {tracksByDisc[discNumber].map((track) => (
              <TrackListItem
                key={track.id}
                track={track}
                isPlaying={currentTrackId === track.id}
                onClick={() => onTrackClick?.(track)}
              />
            ))}
          </div>
        </div>
      ))}

      {/* Empty state */}
      {tracks.length === 0 && (
        <div className="text-center py-8 text-neutral-500 dark:text-neutral-500">
          <p>No tracks found</p>
        </div>
      )}
    </div>
  )
}

export type { TrackListProps } from './TrackList.types'
export { TrackList }
