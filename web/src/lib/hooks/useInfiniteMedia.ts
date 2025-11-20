/**
 * useInfiniteMedia Hook
 * Generic infinite scroll functionality for movies, TV shows, and music
 */

import { useInfiniteQuery } from '@tanstack/react-query'

const PAGE_SIZE = 50

export interface PaginatedResponse {
  pagination?: {
    hasMore?: boolean
    offset?: number
    limit?: number
    total?: number
  }
}

export interface UseInfiniteMediaOptions<TParams, TResponse extends PaginatedResponse> {
  queryKey: (string | number)[]
  queryFn: (
    params: TParams,
    options?: RequestInit
  ) => Promise<{ data: TResponse; status: number; headers: Headers }>
  params: Omit<TParams, 'limit' | 'offset'>
  enabled?: boolean
  pageSize?: number
}

export const useInfiniteMedia = <
  TParams extends { limit?: number; offset?: number },
  TResponse extends PaginatedResponse,
>({
  queryKey,
  queryFn,
  params,
  enabled = true,
  pageSize = PAGE_SIZE,
}: UseInfiniteMediaOptions<TParams, TResponse>) => {
  return useInfiniteQuery({
    queryKey: [...queryKey, 'infinite', params],
    queryFn: async ({ pageParam = 0 }) => {
      const response = await queryFn({
        ...params,
        limit: pageSize,
        offset: pageParam,
      } as TParams)
      return response.data
    },
    getNextPageParam: (lastPage) => {
      // Handle case where lastPage might be undefined or null
      if (!lastPage || !lastPage.pagination) {
        return undefined
      }

      // Use pagination metadata if available
      if (lastPage.pagination.hasMore) {
        const currentOffset = lastPage.pagination.offset ?? 0
        const limit = lastPage.pagination.limit ?? pageSize
        return currentOffset + limit
      }
      return undefined
    },
    initialPageParam: 0,
    enabled,
  })
}

/**
 * Helper to flatten paginated results into a single array
 */
export const flattenPages = <T>(
  pages: Array<{ [key: string]: T[] | undefined }> = [],
  dataKey: string
): T[] => {
  return pages.flatMap((page) => (page[dataKey] as T[]) ?? [])
}
