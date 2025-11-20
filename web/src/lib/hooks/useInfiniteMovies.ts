/**
 * useInfiniteMovies Hook
 * Provides infinite scroll functionality for movies
 */

import { getApiMovies } from '../api/generated/movies/movies'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type { GithubComViewraViewraInternalApplicationMoviesMovieResponse } from '../api/generated/models'

export interface UseInfiniteMoviesOptions {
  libraryId: number
  sort?: string
  enabled?: boolean
  pageSize?: number
}

export function useInfiniteMovies({ libraryId, sort, enabled = true, pageSize }: UseInfiniteMoviesOptions) {
  return useInfiniteMedia({
    queryKey: ['movies', sort],
    queryFn: getApiMovies,
    params: { library_id: libraryId, sort },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export function flattenMovies(
  pages: Array<{
    movies?: GithubComViewraViewraInternalApplicationMoviesMovieResponse[]
  }> = []
) {
  return flattenPages<GithubComViewraViewraInternalApplicationMoviesMovieResponse>(pages, 'movies')
}
