import type { ReactNode } from 'react'

export interface MediaCardProps {
  /**
   * Unique identifier for the media item
   */
  mediaId: number

  /**
   * Type of media (used for image fetching)
   */
  mediaType: 'movie' | 'tv-show' | 'music-album' | 'music-artist'

  /**
   * Alt text for the image
   */
  imageAlt: string

  /**
   * Fallback icon/emoji when image is not available
   */
  imageFallback?: string

  /**
   * Aspect ratio of the card image
   */
  aspectRatio?: '2/3' | 'square'

  /**
   * Click handler for the card
   */
  onClick?: () => void

  /**
   * Play button click handler (for the hover play button)
   * If provided, the play button will be interactive
   */
  onPlay?: () => void

  /**
   * Badge content to display in the top corners
   */
  badges?: ReactNode

  /**
   * Overlay content to display over the image (e.g., progress bars, watched badges)
   * These elements should use absolute positioning
   */
  overlays?: ReactNode

  /**
   * Content to display in the info section below the image
   */
  infoContent?: ReactNode

  /**
   * Icon type for the play button
   */
  playIconType?: 'play' | 'view'
}
