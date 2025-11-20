/**
 * useInfiniteArtists Hook
 * Provides infinite scroll functionality for music artists
 */

import { getApiMusicArtists } from '../api/generated/music/music'
import { useInfiniteMedia, flattenPages } from './useInfiniteMedia'
import type { GithubComViewraViewraInternalApplicationMusicArtistSummary } from '../api/generated/models'

export interface UseInfiniteArtistsOptions {
  libraryId: number
  sort?: string
  enabled?: boolean
  pageSize?: number
}

export function useInfiniteArtists({ libraryId, sort, enabled = true, pageSize }: UseInfiniteArtistsOptions) {
  return useInfiniteMedia({
    queryKey: ['music', 'artists', sort || 'title_asc'],
    queryFn: getApiMusicArtists,
    params: { library_id: libraryId, sort },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export function flattenArtists(
  pages: Array<{
    artists?: GithubComViewraViewraInternalApplicationMusicArtistSummary[]
  }> = []
) {
  return flattenPages<GithubComViewraViewraInternalApplicationMusicArtistSummary>(pages, 'artists')
}
