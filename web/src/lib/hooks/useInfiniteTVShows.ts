/**
 * useInfiniteTVShows Hook
 * Provides infinite scroll functionality for TV shows
 */

import type {
  GithubComMantonxViewraInternalApplicationTvTVShowSummary,
} from '../api/generated/models'
import { getApiTvShows } from '../api/generated/tv/tv'
import { flattenPages, useInfiniteMedia } from './useInfiniteMedia'

export interface UseInfiniteTVShowsOptions {
  libraryId: number
  sort?: string
  search?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteTVShows = ({
  libraryId,
  sort,
  search,
  enabled = true,
  pageSize,
}: UseInfiniteTVShowsOptions) => {
  return useInfiniteMedia({
    // Include search in query key so results are cached separately per search term
    queryKey: ['tv', 'shows', sort || 'title_asc', search || ''],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    queryFn: getApiTvShows as any,
    params: { library_id: libraryId, sort, q: search || undefined },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenTVShows = (
  pages: Array<{
    shows?: GithubComMantonxViewraInternalApplicationTvTVShowSummary[]
  }> = []
) => {
  return flattenPages<GithubComMantonxViewraInternalApplicationTvTVShowSummary>(pages, 'shows')
}
