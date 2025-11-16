/**
 * MediaPoster Component
 * Displays media poster with loading states and fallbacks
 */

import { useState } from 'react'
import { useMovieImages } from '@/lib/hooks/useMediaImages'
import { getPosterImage, getImageUrl } from '@/lib/types/images'

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
   * Additional CSS classes
   */
  className?: string

  /**
   * Fallback emoji/icon to show when no poster is available
   */
  fallbackIcon?: string
}

export const MediaPoster = ({
  mediaId,
  alt,
  className = '',
  fallbackIcon = '🎬',
}: MediaPosterProps) => {
  const { data: imagesData, isLoading } = useMovieImages(mediaId)
  const [imageError, setImageError] = useState(false)

  // Get poster image
  const poster = imagesData?.images ? getPosterImage(imagesData.images) : null
  const posterUrl = poster ? getImageUrl(poster.id) : null

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
