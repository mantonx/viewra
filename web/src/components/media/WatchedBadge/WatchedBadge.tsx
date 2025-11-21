import type { WatchedBadgeProps } from './WatchedBadge.types'

/**
 * Watched badge overlay component for media cards
 * Shows a centered "Watched" badge on media thumbnails
 */
export const WatchedBadge = ({ isWatched }: WatchedBadgeProps) => {
  if (!isWatched) {
    return null
  }

  return (
    <div className="absolute bottom-2 right-2 z-10">
      <div className="bg-green-500 text-white px-3 py-2 rounded-full font-semibold text-sm shadow-lg flex items-center gap-1">
        <span>✓</span>
        <span>Watched</span>
      </div>
    </div>
  )
}
