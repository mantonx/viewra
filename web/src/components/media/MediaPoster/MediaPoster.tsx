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

  /**
   * Aspect ratio for the poster container
   * - '2/3': Standard poster ratio (default for movies/TV)
   * - 'square': 1:1 ratio (default for music albums)
   * - '16/9': Widescreen ratio (for episode thumbnails)
   * Defaults to '2/3' for most media, 'square' for albums, '16/9' for episodes
   */
  aspectRatio?: '2/3' | 'square' | '16/9'
}

export const MediaPoster = ({
  mediaId,
  alt,
  mediaType = 'media',
  className = '',
  fallbackIcon = '🎬',
  preset = 'medium',
  aspectRatio,
}: MediaPosterProps) => {
  const [imageError, setImageError] = useState(false)
  const [imageLoaded, setImageLoaded] = useState(false)

  // Determine aspect ratio based on media type if not explicitly provided
  const resolvedAspectRatio =
    aspectRatio ||
    (mediaType === 'music-album' || mediaType === 'music-artist'
      ? 'square'
      : mediaType === 'tv-episode'
        ? '16/9'
        : '2/3')

  // Convert aspect ratio string to CSS value
  const aspectRatioValue =
    resolvedAspectRatio === 'square' ? '1 / 1' : resolvedAspectRatio === '16/9' ? '16 / 9' : '2 / 3'

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
      ? images && images.length > 0
        ? getEpisodeThumbnail(images)
        : null
      : mediaType === 'music-album'
        ? images && images.length > 0
          ? getAlbumCover(images)
          : null
        : mediaType === 'music-artist'
          ? images && images.length > 0
            ? getArtistImage(images)
            : null
          : images && images.length > 0
            ? getPosterImage(images) || getAlbumCover(images)
            : null
  const imageUrl = image ? getImageUrl(image.id, preset) : null

  // Show loading state
  if (isLoading) {
    return (
      <div
        className={`bg-linear-to-br from-neutral-700 to-neutral-900 flex items-center justify-center animate-pulse ${className}`}
        style={{ aspectRatio: aspectRatioValue }}
      >
        <span className="text-white text-4xl opacity-50">{fallbackIcon}</span>
      </div>
    )
  }

  // Show fallback if no image or image failed to load
  if (!imageUrl || imageError) {
    return (
      <div
        className={`bg-linear-to-br from-neutral-700 to-neutral-900 flex items-center justify-center ${className}`}
        style={{ aspectRatio: aspectRatioValue }}
      >
        <span className="text-white text-4xl">{fallbackIcon}</span>
      </div>
    )
  }

  // Show image with fade-in transition
  return (
    <div className={`relative ${className}`} style={{ aspectRatio: aspectRatioValue }}>
      {/* Placeholder shown while image loads */}
      {!imageLoaded && (
        <div className="absolute inset-0 bg-linear-to-br from-neutral-700 to-neutral-900 flex items-center justify-center animate-pulse">
          <span className="text-white text-4xl opacity-50">{fallbackIcon}</span>
        </div>
      )}

      {/* Actual image with fade-in */}
      <img
        src={imageUrl}
        alt={alt}
        className={`object-cover w-full h-full transition-opacity duration-300 ${
          imageLoaded ? 'opacity-100' : 'opacity-0'
        }`}
        onLoad={() => setImageLoaded(true)}
        onError={() => setImageError(true)}
        loading="lazy"
      />
    </div>
  )
}
