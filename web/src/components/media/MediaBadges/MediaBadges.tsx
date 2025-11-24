import { getCodecBadgeColor } from '@/lib/utils/media'
import type { MediaBadgesProps } from './MediaBadges.types'

export const MediaBadges = ({ preferences, badges }: MediaBadgesProps) => {
  return (
    <div className="flex gap-1 flex-wrap">
      {/* Essential badges - always shown if data exists */}
      {badges.isNew && (
        <span className="px-2 py-1 text-xs font-bold bg-linear-to-r from-green-500 to-emerald-600 text-white rounded shadow-lg">
          NEW
        </span>
      )}

      {/* Optional badges - only shown if preference enabled AND data exists */}
      {preferences.optional.extra && badges.isExtra && (
        <span className="px-2 py-1 text-xs font-semibold bg-yellow-500 text-black rounded">
          EXTRA
        </span>
      )}

      {preferences.optional.resolution && badges.resolution && (
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          {badges.resolution}
        </span>
      )}

      {preferences.optional.contentRating && badges.contentRating && (
        <span className="px-2 py-1 text-xs font-semibold bg-neutral-800 dark:bg-neutral-700 bg-opacity-90 text-white rounded border border-neutral-600 dark:border-neutral-500">
          {badges.contentRating}
        </span>
      )}

      {preferences.optional.codec && badges.codec && (
        <span
          className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(
            badges.codec
          )}`}
        >
          {badges.codec.toUpperCase()}
        </span>
      )}

      {preferences.optional.mediaType && badges.mediaType && (
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          {badges.mediaType}
        </span>
      )}
    </div>
  )
}
