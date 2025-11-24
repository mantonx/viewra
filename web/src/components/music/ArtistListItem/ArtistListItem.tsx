import { MediaListItem } from '@/components/media'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ArtistListItemProps } from './ArtistListItem.types'

/**
 * ArtistListItem - List view representation of a music artist
 * Shows artist image, name, and album/track counts in a horizontal layout
 */
export const ArtistListItem = ({ artist, onClick }: ArtistListItemProps) => {
  return (
    <MediaListItem
      mediaId={artist.id ?? 0}
      mediaType="music-artist"
      title={artist.name ?? 'Artist'}
      imageAlt={`${artist.name ?? 'Artist'} image`}
      imageFallback="🎤"
      aspectRatio="square"
      iconType="view"
      rounded="full"
      onClick={onClick}
      ariaLabel={`View ${artist.name}`}
    >
      <div className="flex-1 min-w-0 flex flex-col justify-center">
        <h3 className={cn('text-lg font-semibold mb-2 truncate', text.primary)}>
          {artist.name}
        </h3>

        {/* Metadata */}
        <div className={cn('flex flex-wrap gap-4 text-sm', text.secondary)}>
          {artist.album_count !== undefined && (
            <span className="flex items-center gap-1">
              <span className="font-medium">{artist.album_count}</span>
              <span>{artist.album_count === 1 ? 'Album' : 'Albums'}</span>
            </span>
          )}
          {artist.track_count !== undefined && (
            <span className="flex items-center gap-1">
              <span className="font-medium">{artist.track_count}</span>
              <span>{artist.track_count === 1 ? 'Track' : 'Tracks'}</span>
            </span>
          )}
        </div>
      </div>
    </MediaListItem>
  )
}
