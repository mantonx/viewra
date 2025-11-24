import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import { MediaMetadata } from '@/components/media/MediaMetadata'
import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick, onPlay }: TVShowCardProps) => {
  const { preferences } = useBadgePreferences()

  // TODO: Enable isNew when backend adds created_at field
  const isNew = false

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
            contentRating: show.content_rating,
            mediaType: 'TV SHOW',
          }}
        />
      }
      infoContent={
        <MediaMetadata
          title={show.title ?? 'Unknown Show'}
          year={show.year}
          genres={show.genre}
          plot={show.plot}
          seasonCount={show.season_count}
          episodeCount={show.episode_count}
          links={{
            imdb: show.imdb_id,
            tmdb: show.tmdb_id ? String(show.tmdb_id) : undefined,
          }}
        />
      }
    />
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
