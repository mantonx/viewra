import { MediaCard } from '@/components/media/MediaCard'
import type { ArtistCardProps } from './ArtistCard.types'

const ArtistCard = ({ artist, onClick }: ArtistCardProps) => {
  return (
    <MediaCard
      mediaId={artist.id}
      mediaType="music-artist"
      imageAlt={artist.name}
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
          <h3 className="font-semibold text-sm line-clamp-2 mb-2">{artist.name}</h3>
          <div className="flex items-center justify-between text-xs text-gray-600">
            <span>
              {artist.album_count} {artist.album_count === 1 ? 'Album' : 'Albums'}
            </span>
            <span>
              {artist.track_count} {artist.track_count === 1 ? 'Track' : 'Tracks'}
            </span>
          </div>
        </>
      }
    />
  )
}

export type { ArtistCardProps } from './ArtistCard.types'
export { ArtistCard }
