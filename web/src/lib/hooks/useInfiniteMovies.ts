/**
 * useInfiniteMovies Hook
 * Provides infinite scroll functionality for movies
 */

import { getApiMovies } from '../api/generated/movies/movies'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type {
  GithubComMantonxViewraInternalApplicationMoviesMovieResponse,
  GithubComMantonxViewraInternalApplicationMoviesListMoviesResponse,
} from '../api/generated/models'

export interface UseInfiniteMoviesOptions {
  libraryId: number
  sort?: string
  search?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteMovies = ({ libraryId, sort, search, enabled = true, pageSize }: UseInfiniteMoviesOptions) => {
  // Wrapper function to ensure proper typing for useInfiniteMedia
  const queryFn = async (
    params: { library_id: number; sort?: string; q?: string; limit?: number; offset?: number },
    options?: RequestInit
  ): Promise<{ data: GithubComMantonxViewraInternalApplicationMoviesListMoviesResponse; status: number; headers: Headers }> => {
    const response = await getApiMovies(params, options)
    return response as { data: GithubComMantonxViewraInternalApplicationMoviesListMoviesResponse; status: number; headers: Headers }
  }

  return useInfiniteMedia({
    // Include search in query key so results are cached separately per search term
    queryKey: ['movies', sort || 'title-asc', search || ''],
    queryFn,
    params: { library_id: libraryId, sort, q: search || undefined },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenMovies = (
  pages: Array<{
    movies?: GithubComMantonxViewraInternalApplicationMoviesMovieResponse[]
  }> = []
) => {
  return flattenPages<GithubComMantonxViewraInternalApplicationMoviesMovieResponse>(pages, 'movies')
}
