/**
 * useInfiniteArtists Hook
 * Provides infinite scroll functionality for music artists
 */

import { getApiMusicArtists } from '../api/generated/music/music'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type {
  GithubComMantonxViewraInternalApplicationMusicArtistSummary,
  GithubComMantonxViewraInternalApplicationMusicListArtistsResponse,
} from '../api/generated/models'

export interface UseInfiniteArtistsOptions {
  libraryId: number
  sort?: string
  search?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteArtists = ({ libraryId, sort, search, enabled = true, pageSize }: UseInfiniteArtistsOptions) => {
  // Wrapper function to ensure proper typing for useInfiniteMedia
  const queryFn = async (
    params: { library_id: number; sort?: string; q?: string; limit?: number; offset?: number },
    options?: RequestInit
  ): Promise<{ data: GithubComMantonxViewraInternalApplicationMusicListArtistsResponse; status: number; headers: Headers }> => {
    const response = await getApiMusicArtists(params, options)
    return response as { data: GithubComMantonxViewraInternalApplicationMusicListArtistsResponse; status: number; headers: Headers }
  }

  return useInfiniteMedia({
    // Include search in query key so results are cached separately per search term
    queryKey: ['music', 'artists', sort || 'title_asc', search || ''],
    queryFn,
    params: { library_id: libraryId, sort, q: search || undefined },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenArtists = (
  pages: Array<{
    artists?: GithubComMantonxViewraInternalApplicationMusicArtistSummary[]
  }> = []
) => {
  return flattenPages<GithubComMantonxViewraInternalApplicationMusicArtistSummary>(pages, 'artists')
}
