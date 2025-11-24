import type { ReactNode } from 'react'

export interface MediaListItemProps {
  /**
   * Media ID for poster lookup
   */
  mediaId: number

  /**
   * Type of media for poster endpoint
   */
  mediaType: 'movie' | 'tv-show' | 'music-artist'

  /**
   * Primary title/name
   */
  title: string

  /**
   * Alt text for poster image
   */
  imageAlt: string

  /**
   * Fallback icon if poster not available
   */
  imageFallback?: string

  /**
   * Aspect ratio of thumbnail
   */
  aspectRatio?: '2/3' | 'square'

  /**
   * Icon type for hover button
   */
  iconType?: 'play' | 'view'

  /**
   * Rounded style for thumbnail
   */
  rounded?: 'default' | 'full'

  /**
   * Optional NEW badge
   */
  showNewBadge?: boolean

  /**
   * Main content area (metadata, plot, etc.)
   */
  children: ReactNode

  /**
   * Click handler
   */
  onClick?: () => void

  /**
   * Aria label for accessibility
   */
  ariaLabel?: string
}
