/**
 * useInfiniteArtists Hook
 * Provides infinite scroll functionality for music artists
 */

import { getApiMusicArtists } from '../api/generated/music/music'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type {
  GithubComViewraViewraInternalApplicationMusicArtistSummary,
  GithubComViewraViewraInternalApplicationMusicListArtistsResponse,
} from '../api/generated/models'

export interface UseInfiniteArtistsOptions {
  libraryId: number
  sort?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteArtists = ({ libraryId, sort, enabled = true, pageSize }: UseInfiniteArtistsOptions) => {
  // Wrapper function to ensure proper typing for useInfiniteMedia
  const queryFn = async (
    params: { library_id: number; sort?: string; limit?: number; offset?: number },
    options?: RequestInit
  ): Promise<{ data: GithubComViewraViewraInternalApplicationMusicListArtistsResponse; status: number; headers: Headers }> => {
    const response = await getApiMusicArtists(params, options)
    return response as { data: GithubComViewraViewraInternalApplicationMusicListArtistsResponse; status: number; headers: Headers }
  }

  return useInfiniteMedia({
    queryKey: ['music', 'artists', sort || 'title_asc'],
    queryFn,
    params: { library_id: libraryId, sort },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenArtists = (
  pages: Array<{
    artists?: GithubComViewraViewraInternalApplicationMusicArtistSummary[]
  }> = []
) => {
  return flattenPages<GithubComViewraViewraInternalApplicationMusicArtistSummary>(pages, 'artists')
}
