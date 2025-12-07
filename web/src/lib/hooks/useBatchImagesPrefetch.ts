/**
 * useBatchImagesPrefetch Hook
 * Coordinates image loading with infinite scroll - prefetches next page images
 * BEFORE they're needed, eliminating visible gaps during scroll
 */

import { useEffect, useRef } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { imagesApi } from '@/lib/api'
import { authFetch } from '@/lib/utils/authFetch'
import type { Image } from '@/lib/types/images'

const BATCH_SIZE = 50

interface UseBatchImagesPrefetchOptions {
  // Current loaded items
  currentItemIds: number[]

  // Media type for entity-based queries
  mediaType?: 'tv_show' | 'tv_season' | 'music_artist' | 'music_album'
  isEntityBased?: boolean

  // Infinite scroll state for predictive prefetching
  hasNextPage?: boolean
  isFetchingNextPage?: boolean

  // Pagination info for fetching next page IDs
  libraryId?: number
  sort?: string
  contentType?: 'movies' | 'tv' | 'music'
}

interface BatchImagesResponse {
  media_images?: Record<number, Image[]>
}

interface IDsResponse {
  ids: number[]
  total: number
  pagination?: {
    limit: number
    offset: number
    total: number
    has_next: boolean
    has_prev: boolean
  }
}

export const useBatchImagesPrefetch = ({
  currentItemIds,
  mediaType,
  isEntityBased = false,
  hasNextPage = false,
  isFetchingNextPage = false,
  libraryId,
  sort,
  contentType,
}: UseBatchImagesPrefetchOptions) => {

  // Track if we've prefetched next page
  const hasPrefetchedRef = useRef(false)

  // Calculate next page offset for predictive prefetching
  const nextPageOffset = currentItemIds.length
  const shouldPrefetch = !isEntityBased && hasNextPage && !isFetchingNextPage &&
                         !!libraryId && !!contentType && nextPageOffset > 0

  // Prefetch next page IDs (only when user is 75% through current page)
  const nextPageIDsQuery = useQuery({
    queryKey: ['prefetch-ids', contentType, libraryId, nextPageOffset, sort],
    queryFn: async (): Promise<IDsResponse> => {
      const endpoint = `/api/${contentType}/ids`
      const params = new URLSearchParams({
        // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
        library_id: libraryId!.toString(),
        limit: BATCH_SIZE.toString(),
        offset: nextPageOffset.toString(),
        ...(sort && { sort }),
      })
      const response = await authFetch(`${endpoint}?${params}`)
      if (!response.ok) {throw new Error('Failed to fetch next page IDs')}
      return response.json()
    },
    enabled: shouldPrefetch && !hasPrefetchedRef.current,
    staleTime: 1000 * 60 * 5, // Cache for 5 minutes
  })

  // Mark as prefetched once we have data
  useEffect(() => {
    if (nextPageIDsQuery.data?.ids.length) {
      hasPrefetchedRef.current = true
    }
  }, [nextPageIDsQuery.data])

  // Reset prefetch flag when offset changes (new page loaded)
  useEffect(() => {
    hasPrefetchedRef.current = false
  }, [nextPageOffset])

  // Combine current IDs + prefetched next page IDs
  const allIds = [...currentItemIds]
  if (nextPageIDsQuery.data?.ids) {
    allIds.push(...nextPageIDsQuery.data.ids)
  }

  // Split all IDs into batches of 50
  const batches: number[][] = []
  for (let i = 0; i < allIds.length; i += BATCH_SIZE) {
    batches.push(allIds.slice(i, i + BATCH_SIZE))
  }

  // Query for all current batches + prefetch next batch
  const queries = useQueries({
    queries: batches.map((batch, _index) => ({
      queryKey: isEntityBased
        ? ['batch-images-entity', mediaType, batch.sort().join(',')]
        : ['batch-images', batch.sort().join(',')],
      queryFn: async () => {
        if (batch.length === 0) {
          return { media_images: {} }
        }

        if (isEntityBased && mediaType) {
          const response = await imagesApi.getBatchEntityImages(batch, mediaType)
          // customInstance returns { data, status, headers }, extract the data
          return (response as unknown as { data: BatchImagesResponse }).data
        } else {
          const response = await imagesApi.getBatchMediaImages(batch)
          // customInstance returns { data, status, headers }, extract the data
          return (response as unknown as { data: BatchImagesResponse }).data
        }
      },
      staleTime: 1000 * 60 * 5, // 5 minutes cache
      refetchOnMount: false,
      refetchOnWindowFocus: false,
    })),
  })

  // Merge all batch results into single images map
  const images: Record<number, Image[]> = {}
  queries.forEach(query => {
    if (query.data?.media_images) {
      Object.assign(images, query.data.media_images)
    }
  })

  const isLoading = queries.some(q => q.isLoading)
  const error = queries.find(q => q.error)?.error as Error | null

  // Only return images for current items (not prefetched next page)
  const currentImages: Record<number, Image[]> = {}
  currentItemIds.forEach(id => {
    if (images[id]) {
      currentImages[id] = images[id]
    }
  })

  return {
    images: currentImages,
    isLoading,
    error,
  }
}
