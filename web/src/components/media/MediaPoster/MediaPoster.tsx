/**
 * MediaPoster Component
 * Displays media poster with loading states and fallbacks
 */

import { useState } from 'react'
import {
  useMediaImages,
  useTVShowImages,
  useTVSeasonImages,
  useMusicAlbumImages,
} from '@/lib/hooks/useMediaImages'
import { getPosterImage, getImageUrl, type ImagePreset } from '@/lib/types/images'

export type MediaType = 'media' | 'tv-show' | 'tv-season' | 'music-album'

export interface MediaPosterProps {
  /**
   * Media ID to fetch poster for
   */
  mediaId: number

  /**
   * Alt text for the image
   */
  alt: string

  /**
   * Media type (media, tv-show, tv-season, music-album)
   * Defaults to 'media'
   */
  mediaType?: MediaType

  /**
   * Additional CSS classes
   */
  className?: string

  /**
   * Fallback emoji/icon to show when no poster is available
   */
  fallbackIcon?: string

  /**
   * Image preset size (thumb, medium, large, xlarge)
   * Defaults to 'medium'
   */
  preset?: ImagePreset
}

export const MediaPoster = ({
  mediaId,
  alt,
  mediaType = 'media',
  className = '',
  fallbackIcon = '🎬',
  preset = 'medium',
}: MediaPosterProps) => {
  // Use the appropriate hook based on media type
  const mediaImagesQuery = useMediaImages(mediaId, { enabled: mediaType === 'media' })
  const tvShowImagesQuery = useTVShowImages(mediaId, { enabled: mediaType === 'tv-show' })
  const tvSeasonImagesQuery = useTVSeasonImages(mediaId, { enabled: mediaType === 'tv-season' })
  const musicAlbumImagesQuery = useMusicAlbumImages(mediaId, {
    enabled: mediaType === 'music-album',
  })

  // Select the active query based on media type
  const { data: imagesData, isLoading } =
    mediaType === 'tv-show'
      ? tvShowImagesQuery
      : mediaType === 'tv-season'
        ? tvSeasonImagesQuery
        : mediaType === 'music-album'
          ? musicAlbumImagesQuery
          : mediaImagesQuery
  const [imageError, setImageError] = useState(false)

  // Get poster image
  const poster = imagesData?.images ? getPosterImage(imagesData.images) : null
  const posterUrl = poster ? getImageUrl(poster.id, preset) : null

  // Show loading state
  if (isLoading) {
    return (
      <div
        className={`bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center animate-pulse ${className}`}
      >
        <span className="text-white text-4xl opacity-50">{fallbackIcon}</span>
      </div>
    )
  }

  // Show fallback if no poster or image failed to load
  if (!posterUrl || imageError) {
    return (
      <div
        className={`bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center ${className}`}
      >
        <span className="text-white text-4xl">{fallbackIcon}</span>
      </div>
    )
  }

  // Show poster image
  return (
    <img
      src={posterUrl}
      alt={alt}
      className={`object-cover ${className}`}
      onError={() => setImageError(true)}
      loading="lazy"
    />
  )
}
