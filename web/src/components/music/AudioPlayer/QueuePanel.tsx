import { useAudioPlayer } from '@/lib/contexts/AudioPlayerContext'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { X, Music2, GripVertical } from 'lucide-react'
import type { MusicTrackResponse } from '@/lib/types/music'

interface QueuePanelProps {
  isOpen: boolean
  onClose: () => void
}

const formatTime = (seconds: number): string => {
  if (!isFinite(seconds)) {
    return '0:00'
  }
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

export const QueuePanel = ({ isOpen, onClose }: QueuePanelProps) => {
  const { queue, currentIndex, currentTrack, removeFromQueue, playQueue } = useAudioPlayer()

  if (!isOpen) {return null}

  const handleTrackClick = (index: number) => {
    playQueue(queue, index)
  }

  const handleRemove = (index: number, e: React.MouseEvent) => {
    e.stopPropagation()
    removeFromQueue(index)
  }

  const totalDuration = queue.reduce((sum, track) => sum + (track.duration || 0), 0)

  return (
    <div className="fixed inset-0 z-[60] flex items-end justify-end pointer-events-none">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm pointer-events-auto animate-in fade-in duration-200"
        onClick={onClose}
      />

      {/* Panel with glass treatment */}
      <div className="relative w-full max-w-md h-[70vh] bg-neutral-900/95 backdrop-blur-xl shadow-2xl pointer-events-auto overflow-hidden flex flex-col border-l border-white/10 animate-in slide-in-from-right duration-300">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-white/10 bg-white/5">
          <div>
            <h2 className="text-lg font-semibold text-white">Queue</h2>
            <p className={cn('text-xs', text.tertiary)}>
              {queue.length} {queue.length === 1 ? 'track' : 'tracks'} • {formatTime(totalDuration)}
            </p>
          </div>
          <button
            onClick={onClose}
            className={cn(
              'p-2 rounded-lg hover:bg-white/10 transition-colors cursor-pointer',
              text.tertiary
            )}
            aria-label="Close queue"
          >
            <X size={20} />
          </button>
        </div>

        {/* Queue list */}
        <div className="flex-1 overflow-y-auto">
          {queue.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full p-8">
              <Music2 size={48} className="text-neutral-600" />
              <p className="mt-4 text-sm text-neutral-400">No tracks in queue</p>
            </div>
          ) : (
            <div className="divide-y divide-white/5">
              {queue.map((track: MusicTrackResponse, index: number) => {
                const isCurrentTrack = index === currentIndex && track.id === currentTrack?.id

                return (
                  <div
                    key={`${track.id}-${index}`}
                    onClick={() => handleTrackClick(index)}
                    className={cn(
                      'flex items-center gap-3 p-3 cursor-pointer transition-all group',
                      isCurrentTrack
                        ? 'bg-primary-500/15 border-l-2 border-primary-500'
                        : 'hover:bg-white/5'
                    )}
                  >
                    {/* Drag handle (visual only for now) */}
                    <div className={cn('flex-shrink-0', text.tertiary, 'opacity-0 group-hover:opacity-100')}>
                      <GripVertical size={16} />
                    </div>

                    {/* Track number or playing indicator */}
                    <div className="w-6 flex-shrink-0 text-center">
                      {isCurrentTrack ? (
                        <div className="flex items-center justify-center">
                          <div className="w-1 h-3 bg-primary-500 rounded-full animate-pulse" />
                        </div>
                      ) : (
                        <span className={cn('text-xs', text.tertiary)}>{index + 1}</span>
                      )}
                    </div>

                    {/* Track info */}
                    <div className="flex-1 min-w-0">
                      <p
                        className={cn(
                          'text-sm font-medium truncate',
                          isCurrentTrack ? 'text-primary-500' : 'text-white'
                        )}
                      >
                        {track.title}
                      </p>
                      <p className={cn('text-xs truncate', text.tertiary)}>
                        {track.artist || 'Unknown Artist'}
                        {track.album && ` • ${track.album}`}
                      </p>
                    </div>

                    {/* Duration */}
                    <span className={cn('text-xs flex-shrink-0', text.tertiary)}>
                      {formatTime(track.duration)}
                    </span>

                    {/* Remove button */}
                    <button
                      onClick={(e) => handleRemove(index, e)}
                      className="p-1.5 rounded-lg hover:bg-white/10 transition-all opacity-0 group-hover:opacity-100 text-neutral-500 hover:text-red-400 cursor-pointer"
                      aria-label="Remove from queue"
                    >
                      <X size={16} />
                    </button>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
