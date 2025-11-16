/**
 * TV Shows API Client
 * Direct API calls for TV show endpoints
 */

import { customInstance } from './mutator'
import type { ListTVShowsResponse, ListTVEpisodesResponse, TVEpisodeResponse, TVShowDetailResponse } from '@/lib/types/tv'

export const tvApi = {
  /**
   * List all TV shows for a library
   */
  listShows: (libraryId: number) =>
    customInstance<ListTVShowsResponse>({
      url: `/api/tv/shows?library_id=${libraryId}`,
      method: 'GET',
    }),

  /**
   * Get a specific TV show by ID
   */
  getShow: (showId: number) =>
    customInstance<TVShowDetailResponse>({
      url: `/api/tv/shows/${showId}`,
      method: 'GET',
    }),

  /**
   * List all episodes for a specific show by ID
   */
  listEpisodesByShowId: (showId: number) =>
    customInstance<ListTVEpisodesResponse>({
      url: `/api/tv/shows/${showId}/episodes`,
      method: 'GET',
    }),

  /**
   * List all episodes for a specific show by title (deprecated, use listEpisodesByShowId)
   */
  listEpisodesByShow: (libraryId: number, showTitle: string) =>
    customInstance<ListTVEpisodesResponse>({
      url: `/api/tv/shows/${encodeURIComponent(showTitle)}/episodes?library_id=${libraryId}`,
      method: 'GET',
    }),

  /**
   * Get a specific episode by ID
   */
  getEpisode: (id: number) =>
    customInstance<TVEpisodeResponse>({
      url: `/api/tv/episodes/${id}`,
      method: 'GET',
    }),

  /**
   * Search TV episodes
   */
  search: (libraryId: number, query: string) =>
    customInstance<ListTVEpisodesResponse>({
      url: `/api/tv/search?library_id=${libraryId}&q=${encodeURIComponent(query)}`,
      method: 'GET',
    }),
}
