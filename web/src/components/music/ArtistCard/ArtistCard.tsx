import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import type { ArtistCardProps } from './ArtistCard.types'

const ArtistCard = ({ artist, onClick }: ArtistCardProps) => {
  const { preferences } = useBadgePreferences()

  // Check if artist is newly added (within last 7 days)
  const isNew =
    artist.created_at &&
    Date.now() - new Date(artist.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={artist.id ?? 0}
      mediaType="music-artist"
      imageAlt={artist.name ?? 'Artist'}
      imageFallback="🎤"
      aspectRatio="square"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            mediaType: 'ARTIST', // Only shown if user enables mediaType badge
          }}
        />
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-2 text-neutral-900 dark:text-neutral-50">
            {artist.name ?? 'Unknown Artist'}
          </h3>
          <div className="flex items-center justify-between text-xs text-neutral-600 dark:text-neutral-400">
            <span>
              {artist.album_count ?? 0} {artist.album_count === 1 ? 'Album' : 'Albums'}
            </span>
            <span>
              {artist.track_count ?? 0} {artist.track_count === 1 ? 'Track' : 'Tracks'}
            </span>
          </div>
        </>
      }
    />
  )
}

export type { ArtistCardProps } from './ArtistCard.types'
export { ArtistCard }
