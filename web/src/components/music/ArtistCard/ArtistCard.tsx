import { MediaPoster } from '@/components/media/MediaPoster'
import type { ArtistCardProps } from './ArtistCard.types'

const ArtistCard = ({ artist, onClick }: ArtistCardProps) => {
  const handleClick = () => {
    onClick?.()
  }

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
      onClick={handleClick}
    >
      {/* Thumbnail */}
      <div className="aspect-square relative">
        <MediaPoster
          mediaId={artist.id}
          mediaType="music-artist"
          alt={artist.name}
          className="w-full h-full absolute inset-0"
          preset="medium"
          fallbackIcon="🎤"
        />
        {/* Badge overlay */}
        <div className="absolute top-2 left-2 right-2">
          <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
            ARTIST
          </span>
        </div>
      </div>

      {/* Info */}
      <div className="p-3">
        <h3 className="font-semibold text-sm line-clamp-2 mb-2">{artist.name}</h3>
        <div className="flex items-center justify-between text-xs text-gray-600">
          <span>
            {artist.album_count} {artist.album_count === 1 ? 'Album' : 'Albums'}
          </span>
          <span>
            {artist.track_count} {artist.track_count === 1 ? 'Track' : 'Tracks'}
          </span>
        </div>
      </div>
    </div>
  )
}

export type { ArtistCardProps } from './ArtistCard.types'
export { ArtistCard }
