/**
 * Movies API Client
 * Wrapper around generated API functions
 */

import { getApiMovies, getApiMoviesId, getApiMoviesSearch } from './generated/movies/movies'
import type { GetApiMoviesParams, GetApiMoviesSearchParams } from './generated/models'

export const moviesApi = {
  /**
   * List all movies for a library with optional pagination
   */
  listMovies: (params: GetApiMoviesParams) => getApiMovies(params),

  /**
   * Get a specific movie by ID
   */
  getMovie: (id: number) => getApiMoviesId(id),

  /**
   * Search movies with optional pagination
   */
  search: (params: GetApiMoviesSearchParams) => getApiMoviesSearch(params),
}
