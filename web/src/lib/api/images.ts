/**
 * Images API Client
 * Direct API calls for image endpoints
 */

import { customInstance } from './mutator'
import type { ImageResponse, ListImagesResponse } from '@/lib/types/images'

export const imagesApi = {
  /**
   * Get image metadata by ID
   */
  getImage: (id: number) =>
    customInstance<ImageResponse>({
      url: `/api/images/${id}`,
      method: 'GET',
    }),

  /**
   * Get all images for a media item
   */
  getMediaImages: (mediaId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/media/${mediaId}/images`,
      method: 'GET',
    }),

  /**
   * Get all images for a movie
   */
  getMovieImages: (movieId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/movies/${movieId}/images`,
      method: 'GET',
    }),

  /**
   * Get all images for a TV episode
   */
  getEpisodeImages: (episodeId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/tv/episodes/${episodeId}/images`,
      method: 'GET',
    }),

  /**
   * Get all images for a TV show
   */
  getTVShowImages: (showId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/tv/shows/${showId}/images`,
      method: 'GET',
    }),

  /**
   * Get all images for a TV season
   */
  getTVSeasonImages: (seasonId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/tv/seasons/${seasonId}/images`,
      method: 'GET',
    }),

  /**
   * Get all images for a music album
   */
  getMusicAlbumImages: (albumId: number) =>
    customInstance<ListImagesResponse>({
      url: `/api/music/albums/${albumId}/images`,
      method: 'GET',
    }),
}
