/**
 * MediaPoster Component
 * Displays media poster with loading states and fallbacks
 *
 * Supports batch loading when used within a BatchImagesProvider for performance,
 * falls back to individual image requests when not in a batch context.
 */

import { useState } from 'react'
import {
  useMediaImages,
  useMovieImages,
  useTVShowImages,
  useTVSeasonImages,
  useEpisodeImages,
  useMusicAlbumImages,
  useMusicArtistImages,
} from '@/lib/hooks/useMediaImages'
import { useBatchImagesIfAvailable } from '@/lib/hooks/useBatchImages'
import {
  getPosterImage,
  getEpisodeThumbnail,
  getAlbumCover,
  getArtistImage,
  getImageUrl,
  type ImagePreset,
} from '@/lib/types/images'

export type MediaType = 'media' | 'movie' | 'tv-show' | 'tv-season' | 'tv-episode' | 'music-album' | 'music-artist'

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
   * Media type (media, tv-show, tv-season, tv-episode, music-album, music-artist)
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
  const [imageError, setImageError] = useState(false)

  // Try to use batch images if available (within BatchImagesProvider)
  // Returns null if not in a batch context, allowing graceful fallback to individual queries
  const batchResult = useBatchImagesIfAvailable(mediaId)
  const batchImages = batchResult?.images || null
  const batchLoading = batchResult?.isLoading || false

  // Fallback: Use individual image queries if not in batch context
  const mediaImagesQuery = useMediaImages(mediaId, { enabled: !batchImages && mediaType === 'media' })
  const movieImagesQuery = useMovieImages(mediaId, { enabled: !batchImages && mediaType === 'movie' })
  const tvShowImagesQuery = useTVShowImages(mediaId, { enabled: !batchImages && mediaType === 'tv-show' })
  const tvSeasonImagesQuery = useTVSeasonImages(mediaId, { enabled: !batchImages && mediaType === 'tv-season' })
  const episodeImagesQuery = useEpisodeImages(mediaId, { enabled: !batchImages && mediaType === 'tv-episode' })
  const musicAlbumImagesQuery = useMusicAlbumImages(mediaId, {
    enabled: !batchImages && mediaType === 'music-album',
  })
  const musicArtistImagesQuery = useMusicArtistImages(mediaId, {
    enabled: !batchImages && mediaType === 'music-artist',
  })

  // Determine images and loading state
  let images = batchImages
  let isLoading = batchLoading

  if (!batchImages) {
    // Use individual query results
    const activeQuery =
      mediaType === 'movie'
        ? movieImagesQuery
        : mediaType === 'tv-show'
          ? tvShowImagesQuery
          : mediaType === 'tv-season'
            ? tvSeasonImagesQuery
            : mediaType === 'tv-episode'
              ? episodeImagesQuery
              : mediaType === 'music-album'
                ? musicAlbumImagesQuery
                : mediaType === 'music-artist'
                  ? musicArtistImagesQuery
                  : mediaImagesQuery

    images = activeQuery.data?.images || []
    isLoading = activeQuery.isLoading
  }

  // Get image - use appropriate type based on media type
  const image =
    mediaType === 'tv-episode'
      ? images.length > 0
        ? getEpisodeThumbnail(images)
        : null
      : mediaType === 'music-album'
        ? images.length > 0
          ? getAlbumCover(images)
          : null
        : mediaType === 'music-artist'
          ? images.length > 0
            ? getArtistImage(images)
            : null
          : images.length > 0
            ? getPosterImage(images)
            : null
  const imageUrl = image ? getImageUrl(image.id, preset) : null

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

  // Show fallback if no image or image failed to load
  if (!imageUrl || imageError) {
    return (
      <div
        className={`bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center ${className}`}
      >
        <span className="text-white text-4xl">{fallbackIcon}</span>
      </div>
    )
  }

  // Show image
  return (
    <img
      src={imageUrl}
      alt={alt}
      className={`object-cover ${className}`}
      onError={() => setImageError(true)}
      loading="lazy"
    />
  )
}
