/**
 * TV Shows API Client
 * Wrapper around generated API functions
 */

import { getApiTvShows, getApiTvShowsId, getApiTvEpisodesId, getApiTvShowsIdEpisodes, getApiTvSearch } from './generated/tv/tv'
import type { GetApiTvShowsParams, GetApiTvSearchParams } from './generated/models'

export const tvApi = {
  /**
   * List all TV shows for a library with optional pagination
   */
  listShows: (params: GetApiTvShowsParams) => getApiTvShows(params),

  /**
   * Get a specific TV show by ID
   */
  getShow: (showId: number) => getApiTvShowsId(showId),

  /**
   * List all episodes for a specific show by ID
   */
  listEpisodesByShowId: (showId: number) => getApiTvShowsIdEpisodes(showId),

  /**
   * Get a specific episode by ID
   */
  getEpisode: (id: number) => getApiTvEpisodesId(id),

  /**
   * Search TV episodes with optional pagination
   */
  search: (params: GetApiTvSearchParams) => getApiTvSearch(params),

  /**
   * Get the next episode to watch for a show
   * Returns the in-progress episode if exists, otherwise first unwatched episode,
   * or first episode if all are watched
   */
  getNextEpisode: async (showId: number) => {
    const response = await fetch(`/api/tv/shows/${showId}/next-episode`)
    if (!response.ok) {
      throw new Error(`Failed to get next episode: ${response.statusText}`)
    }
    return response.json()
  },
}
