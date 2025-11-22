/**
 * useInfiniteMovies Hook
 * Provides infinite scroll functionality for movies
 */

import { getApiMovies } from '../api/generated/movies/movies'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type {
  GithubComViewraViewraInternalApplicationMoviesMovieResponse,
  GithubComViewraViewraInternalApplicationMoviesListMoviesResponse,
} from '../api/generated/models'

export interface UseInfiniteMoviesOptions {
  libraryId: number
  sort?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteMovies = ({ libraryId, sort, enabled = true, pageSize }: UseInfiniteMoviesOptions) => {
  // Wrapper function to ensure proper typing for useInfiniteMedia
  const queryFn = async (
    params: { library_id: number; sort?: string; limit?: number; offset?: number },
    options?: RequestInit
  ): Promise<{ data: GithubComViewraViewraInternalApplicationMoviesListMoviesResponse; status: number; headers: Headers }> => {
    const response = await getApiMovies(params, options)
    return response as { data: GithubComViewraViewraInternalApplicationMoviesListMoviesResponse; status: number; headers: Headers }
  }

  return useInfiniteMedia({
    queryKey: ['movies', sort || 'title-asc'],
    queryFn,
    params: { library_id: libraryId, sort },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenMovies = (
  pages: Array<{
    movies?: GithubComViewraViewraInternalApplicationMoviesMovieResponse[]
  }> = []
) => {
  return flattenPages<GithubComViewraViewraInternalApplicationMoviesMovieResponse>(pages, 'movies')
}
