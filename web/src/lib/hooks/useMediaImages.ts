/**
 * useMediaImages Hook
 * Fetches and manages images for media items
 */

import { useQuery } from '@tanstack/react-query'
import { imagesApi } from '@/lib/api'
import type { Image } from '@/lib/types/images'

export interface UseMediaImagesOptions {
  /**
   * Whether to enable the query (default: true)
   */
  enabled?: boolean
}

/**
 * Hook to fetch images for a media item
 */
export function useMediaImages(mediaId: number | undefined, options: UseMediaImagesOptions = {}) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['media-images', mediaId],
    queryFn: async () => {
      if (!mediaId) {
        return { images: [] }
      }
      return imagesApi.getMediaImages(mediaId)
    },
    enabled: enabled && mediaId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Hook to fetch images for a movie
 */
export function useMovieImages(movieId: number | undefined, options: UseMediaImagesOptions = {}) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['movie-images', movieId],
    queryFn: async () => {
      if (!movieId) {
        return { images: [] }
      }
      return imagesApi.getMovieImages(movieId)
    },
    enabled: enabled && movieId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Hook to fetch images for a TV episode
 */
export function useEpisodeImages(
  episodeId: number | undefined,
  options: UseMediaImagesOptions = {}
) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['episode-images', episodeId],
    queryFn: async () => {
      if (!episodeId) {
        return { images: [] }
      }
      return imagesApi.getEpisodeImages(episodeId)
    },
    enabled: enabled && episodeId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Hook to fetch images for a TV show
 */
export function useTVShowImages(
  showId: number | undefined,
  options: UseMediaImagesOptions = {}
) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['tv-show-images', showId],
    queryFn: async () => {
      if (!showId) {
        return { images: [] }
      }
      return imagesApi.getTVShowImages(showId)
    },
    enabled: enabled && showId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Hook to fetch images for a TV season
 */
export function useTVSeasonImages(
  seasonId: number | undefined,
  options: UseMediaImagesOptions = {}
) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['tv-season-images', seasonId],
    queryFn: async () => {
      if (!seasonId) {
        return { images: [] }
      }
      return imagesApi.getTVSeasonImages(seasonId)
    },
    enabled: enabled && seasonId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Hook to fetch images for a music album
 */
export function useMusicAlbumImages(
  albumId: number | undefined,
  options: UseMediaImagesOptions = {}
) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['music-album-images', albumId],
    queryFn: async () => {
      if (!albumId) {
        return { images: [] }
      }
      return imagesApi.getMusicAlbumImages(albumId)
    },
    enabled: enabled && albumId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

/**
 * Helper hook to get poster image URL from media images
 */
export function usePosterUrl(mediaId: number | undefined): string | null {
  const { data } = useMediaImages(mediaId)

  if (!data?.images || data.images.length === 0) {
    return null
  }

  // Find poster image
  const poster = data.images.find((img: Image) => img.image_type === 'poster')
  if (!poster) {
    return null
  }

  return `/api/images/${poster.id}/file`
}
