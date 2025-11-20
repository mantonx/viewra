/**
 * useInfiniteTVShows Hook
 * Provides infinite scroll functionality for TV shows
 */

import type {
  GithubComViewraViewraInternalApplicationTvTVShowSummary,
} from '../api/generated/models'
import { getApiTvShows } from '../api/generated/tv/tv'
import { flattenPages, useInfiniteMedia } from './useInfiniteMedia'

export interface UseInfiniteTVShowsOptions {
  libraryId: number
  sort?: string
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteTVShows = ({
  libraryId,
  sort,
  enabled = true,
  pageSize,
}: UseInfiniteTVShowsOptions) => {
  return useInfiniteMedia({
    queryKey: ['tv', 'shows', sort || 'title_asc'],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    queryFn: getApiTvShows as any,
    params: { library_id: libraryId, sort },
    enabled,
    pageSize,
  })
}

/**
 * Helper to flatten all pages into a single array
 */
export const flattenTVShows = (
  pages: Array<{
    shows?: GithubComViewraViewraInternalApplicationTvTVShowSummary[]
  }> = []
) => {
  return flattenPages<GithubComViewraViewraInternalApplicationTvTVShowSummary>(pages, 'shows')
}
