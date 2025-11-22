import { MediaCard } from '@/components/media/MediaCard'
import type { ArtistCardProps } from './ArtistCard.types'

const ArtistCard = ({ artist, onClick }: ArtistCardProps) => {
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
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          ARTIST
        </span>
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-2">{artist.name ?? 'Unknown Artist'}</h3>
          <div className="flex items-center justify-between text-xs text-gray-600">
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
