import { getCodecBadgeColor } from '@/lib/utils/media'
import type { MediaBadgesProps } from './MediaBadges.types'

// Glass badge base styles
const glassBadge = 'px-2 py-0.5 text-[11px] font-semibold rounded backdrop-blur-md'

// Resolution-specific colors (premium look)
const getResolutionStyle = (resolution: string) => {
  if (resolution === '4K' || resolution === 'UHD') {
    return 'bg-amber-500/90 text-amber-950 border border-amber-400/50'
  }
  if (resolution === '1080p' || resolution === 'FHD') {
    return 'bg-white/90 text-neutral-800 border border-white/50'
  }
  // 720p, SD, etc.
  return 'bg-black/70 text-white border border-white/10'
}

export const MediaBadges = ({ preferences, badges }: MediaBadgesProps) => {
  return (
    <div className="flex gap-1.5 flex-wrap">
      {/* Essential badges - always shown if data exists */}
      {badges.isNew && (
        <span className={`${glassBadge} bg-linear-to-r from-emerald-500 to-green-600 text-white border border-emerald-400/30 shadow-lg shadow-emerald-500/20`}>
          NEW
        </span>
      )}

      {/* Optional badges - only shown if preference enabled AND data exists */}
      {preferences.optional.extra && badges.isExtra && (
        <span className={`${glassBadge} bg-amber-400/90 text-amber-950 border border-amber-300/50`}>
          EXTRA
        </span>
      )}

      {preferences.optional.resolution && badges.resolution && (
        <span className={`${glassBadge} ${getResolutionStyle(badges.resolution)}`}>
          {badges.resolution}
        </span>
      )}

      {preferences.optional.contentRating && badges.contentRating && (
        <span className={`${glassBadge} bg-black/60 text-white border border-white/20`}>
          {badges.contentRating}
        </span>
      )}

      {preferences.optional.codec && badges.codec && (
        <span
          className={`${glassBadge} text-white border border-white/10 ${getCodecBadgeColor(badges.codec)}`}
        >
          {badges.codec.toUpperCase()}
        </span>
      )}

      {preferences.optional.mediaType && badges.mediaType && (
        <span className={`${glassBadge} bg-black/60 text-white border border-white/20`}>
          {badges.mediaType}
        </span>
      )}
    </div>
  )
}
