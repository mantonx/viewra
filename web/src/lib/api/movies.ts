/**
 * Movies API Client
 * Direct API calls for movie endpoints
 */

import { customInstance } from './mutator'
import type { ListMoviesResponse, MovieResponse } from '@/lib/types/movies'

export const moviesApi = {
  /**
   * List all movies for a library
   */
  listMovies: (libraryId: number) =>
    customInstance<ListMoviesResponse>({
      url: `/api/movies?library_id=${libraryId}`,
      method: 'GET',
    }),

  /**
   * Get a specific movie by ID
   */
  getMovie: (id: number) =>
    customInstance<MovieResponse>({
      url: `/api/movies/${id}`,
      method: 'GET',
    }),

  /**
   * Search movies
   */
  search: (libraryId: number, query: string) =>
    customInstance<ListMoviesResponse>({
      url: `/api/movies/search?library_id=${libraryId}&q=${encodeURIComponent(query)}`,
      method: 'GET',
    }),
}
