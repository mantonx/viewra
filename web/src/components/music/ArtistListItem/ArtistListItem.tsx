import { useState } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import { HoverPlayButton } from '@/components/common'
import type { ArtistListItemProps } from './ArtistListItem.types'

/**
 * ArtistListItem - List view representation of a music artist
 * Shows artist image, name, and album/track counts in a horizontal layout
 */
export const ArtistListItem = ({ artist, onClick }: ArtistListItemProps) => {
  const [isHovered, setIsHovered] = useState(false)

  const handleClick = () => {
    if (onClick) {
      onClick()
    }
  }

  return (
    <div
      className="group flex gap-4 p-4 bg-white rounded-lg border-2 border-transparent shadow hover:shadow-lg hover:border-rose-500 transition-all duration-200 cursor-pointer focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-2"
      onClick={handleClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          handleClick()
        }
      }}
      aria-label={`View ${artist.name}`}
    >
      {/* Artist Image with Centered View Button */}
      <div className="shrink-0 relative w-24 h-24">
        <MediaPoster
          mediaType="music-artist"
          mediaId={artist.id ?? 0}
          alt={`${artist.name ?? 'Artist'} image`}
          className="w-full h-full rounded-full shadow-sm transition-all duration-200"
          preset="thumb"
          fallbackIcon="🎤"
        />

        <HoverPlayButton isParentHovered={isHovered} iconType="view" size="small" rounded="full" />
      </div>

      {/* Artist Info */}
      <div className="flex-1 min-w-0 flex flex-col justify-center">
        <h3 className="text-lg font-semibold text-gray-900 mb-2 truncate">
          {artist.name}
        </h3>

        {/* Metadata */}
        <div className="flex flex-wrap gap-4 text-sm text-gray-600">
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
    </div>
  )
}
