import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import { MediaMetadata } from '@/components/media/MediaMetadata'
import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick, onPlay }: TVShowCardProps) => {
  const { preferences } = useBadgePreferences()

  // Check if show is newly added (within last 7 days)
  const isNew =
    show.created_at &&
    Date.now() - new Date(show.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={show.id ?? 0}
      mediaType="tv-show"
      imageAlt={show.title ?? 'TV Show'}
      imageFallback="📺"
      aspectRatio="2/3"
      onClick={onClick}
      onPlay={onPlay}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            // Future: Add resolution, rating, codec once backend provides data
          }}
        />
      }
      infoContent={
        <MediaMetadata
          title={show.title ?? 'Unknown Show'}
          year={show.year} // NEW - needs backend
          genres={show.genre ? [show.genre] : undefined} // NEW - needs backend
          plot={show.plot} // NEW - needs backend
          seasonCount={show.season_count}
          episodeCount={show.episode_count}
          links={{
            imdb: show.imdb_id, // NEW - needs backend
            tmdb: show.tmdb_id ? String(show.tmdb_id) : undefined, // NEW - needs backend
          }}
        />
      }
    />
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
