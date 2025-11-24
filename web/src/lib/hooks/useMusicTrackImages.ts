/**
 * useMusicTrackImages Hook
 * Fetches track images with fallback to album images
 *
 * This implements the hybrid approach:
 * 1. Try to fetch track-level images (embedded album art)
 * 2. If no track images exist, fall back to album images
 */

import { useQuery } from '@tanstack/react-query'
import { imagesApi } from '@/lib/api'
import type { Image } from '@/lib/types/images'

export interface UseMusicTrackImagesOptions {
  /**
   * Track media ID
   */
  trackId: number | undefined

  /**
   * Album ID for fallback
   */
  albumId?: number | undefined

  /**
   * Whether to enable the query (default: true)
   */
  enabled?: boolean
}

/**
 * Hook to fetch images for a music track with album fallback
 */
export const useMusicTrackImages = ({ trackId, albumId, enabled = true }: UseMusicTrackImagesOptions) => {
  return useQuery({
    queryKey: ['music-track-images-with-fallback', trackId, albumId],
    queryFn: async () => {
      if (!trackId) {
        return { images: [], source: 'none' as const }
      }

      // First, try to get track-level images
      try {
        const trackResponse = await imagesApi.getMediaImages(trackId)
        const trackData = (trackResponse as unknown as { data: { images: Image[] } }).data

        if (trackData.images && trackData.images.length > 0) {
          // Track has its own images (embedded album art)
          return { images: trackData.images, source: 'track' as const }
        }
      } catch (error) {
        // Track images not found, continue to fallback
        console.debug('No track images found, falling back to album images')
      }

      // Fallback: Try to get album images if albumId is provided
      if (albumId) {
        try {
          const albumResponse = await imagesApi.getMusicAlbumImages(albumId)
          const albumData = (albumResponse as unknown as { data: { images: Image[] } }).data

          if (albumData.images && albumData.images.length > 0) {
            return { images: albumData.images, source: 'album' as const }
          }
        } catch (error) {
          console.debug('No album images found')
        }
      }

      // No images found at all
      return { images: [], source: 'none' as const }
    },
    enabled: enabled && trackId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}
