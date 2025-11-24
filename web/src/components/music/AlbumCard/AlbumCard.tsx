import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import type { AlbumCardProps } from './AlbumCard.types'

const AlbumCard = ({ album, onClick }: AlbumCardProps) => {
  const { preferences } = useBadgePreferences()

  // Check if album is newly added (within last 7 days)
  const isNew =
    album.created_at &&
    Date.now() - new Date(album.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={album.id ?? 0}
      mediaType="music-album"
      imageAlt={album.album ?? 'Album'}
      imageFallback="💿"
      aspectRatio="square"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            mediaType: 'ALBUM', // Only shown if user enables mediaType badge
          }}
        />
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-1 text-neutral-900 dark:text-neutral-50">
            {album.album ?? 'Unknown Album'}
          </h3>
          <p className="text-xs text-neutral-600 dark:text-neutral-400 mb-2 line-clamp-1">
            {album.artist ?? 'Unknown Artist'}
          </p>
          <div className="flex items-center justify-between text-xs">
            <span className="text-neutral-500 dark:text-neutral-500">
              {album.track_count ?? 0} {album.track_count === 1 ? 'Track' : 'Tracks'}
            </span>
            {album.year && (
              <span className="px-2 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                {album.year}
              </span>
            )}
          </div>
        </>
      }
    />
  )
}

export type { AlbumCardProps } from './AlbumCard.types'
export { AlbumCard }
