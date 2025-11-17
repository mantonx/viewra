import { MediaPoster } from '@/components/media/MediaPoster'
import type { AlbumCardProps } from './AlbumCard.types'

const AlbumCard = ({ album, onClick }: AlbumCardProps) => {
  const handleClick = () => {
    onClick?.()
  }

  return (
    <div
      className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-xl hover:scale-105 transition-all"
      onClick={handleClick}
    >
      {/* Album art */}
      <div className="aspect-square relative">
        <MediaPoster
          mediaId={album.id}
          mediaType="music-album"
          alt={album.album}
          className="w-full h-full absolute inset-0"
          preset="medium"
          fallbackIcon="💿"
        />
        {/* Badge overlays */}
        <div className="absolute top-2 left-2 right-2 flex justify-between items-start">
          <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
            ALBUM
          </span>
          {album.year && (
            <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
              {album.year}
            </span>
          )}
        </div>
      </div>

      {/* Info */}
      <div className="p-3">
        <h3 className="font-semibold text-sm line-clamp-2 mb-1 text-gray-900">{album.album}</h3>
        <p className="text-xs text-gray-600 mb-2 line-clamp-1">{album.artist}</p>
        <div className="flex items-center justify-between text-xs">
          <span className="text-gray-500">
            {album.track_count} {album.track_count === 1 ? 'Track' : 'Tracks'}
          </span>
          {album.year && (
            <span className="px-2 py-0.5 text-[10px] font-medium bg-blue-100 text-blue-700 rounded">
              {album.year}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

export type { AlbumCardProps } from './AlbumCard.types'
export { AlbumCard }
