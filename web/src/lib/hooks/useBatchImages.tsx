/**
 * useBatchImages Hook & Context
 * Provides efficient batch image loading to eliminate N+1 queries
 *
 * Usage:
 * 1. For media items (movies, episodes): <BatchImagesProvider mediaIds={[1,2,3,...]}>
 * 2. For entities (TV shows, artists): <BatchImagesProvider entityIds={[1,2,3,...]} mediaType="tv_show">
 * 3. Use useBatchImages(id) in child components to get images
 */

import { createContext, useContext, useMemo } from 'react'
import type { ReactNode } from 'react'
import { useQueries } from '@tanstack/react-query'
import { imagesApi } from '@/lib/api'
import type { Image } from '@/lib/types/images'

interface BatchImagesContextValue {
  images: Record<number, Image[]>
  isLoading: boolean
  error: Error | null
}

const BatchImagesContext = createContext<BatchImagesContextValue | null>(null)

interface BatchImagesProviderProps {
  mediaIds?: number[]
  entityIds?: number[]
  mediaType?: 'tv_show' | 'tv_season' | 'music_artist' | 'music_album'
  children: ReactNode
}

/**
 * Provider component that fetches images for multiple media items or entities in batched chunks
 * Handles infinite scroll by splitting IDs into 50-item batches and merging results
 */
export const BatchImagesProvider = ({ mediaIds, entityIds, mediaType, children }: BatchImagesProviderProps) => {
  const ids = useMemo(() => mediaIds || entityIds || [], [mediaIds, entityIds])
  const isEntityBased = !!entityIds && !!mediaType

  // Split IDs into chunks of 50 to avoid overwhelming the backend
  const BATCH_SIZE = 50
  const batches = useMemo(() => {
    const chunks: number[][] = []
    for (let i = 0; i < ids.length; i += BATCH_SIZE) {
      chunks.push(ids.slice(i, i + BATCH_SIZE))
    }
    return chunks
  }, [ids]) // Only recalculate when the actual IDs change

  // Use useQueries to fetch all batches in parallel and cache them independently
  const queries = useQueries({
    queries: batches.map((batch, _index) => ({
      queryKey: isEntityBased
        ? ['batch-images-entity', mediaType, batch.sort().join(',')]
        : ['batch-images', batch.sort().join(',')],
      queryFn: async () => {
        if (batch.length === 0) {
          return { media_images: {} }
        }

        if (isEntityBased) {
          // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
          const response = await imagesApi.getBatchEntityImages(batch, mediaType!)
          // customInstance returns { data, status, headers }, extract the data
          return (response as unknown as { data: { media_images: Record<number, Image[]> } }).data
        } else {
          const response = await imagesApi.getBatchMediaImages(batch)
          // customInstance returns { data, status, headers }, extract the data
          return (response as unknown as { data: { media_images: Record<number, Image[]> } }).data
        }
      },
      staleTime: 1000 * 60 * 5, // 5 minutes - each batch stays cached
    })),
  })

  // Merge all batch results into a single images map
  const mergedImages = useMemo(() => {
    const result: Record<number, Image[]> = {}
    queries.forEach(query => {
      if (query.data?.media_images) {
        Object.assign(result, query.data.media_images)
      }
    })
    return result
  }, [queries])

  const isLoading = queries.some(q => q.isLoading)
  const error = queries.find(q => q.error)?.error as Error | null

  const value: BatchImagesContextValue = {
    images: mergedImages,
    isLoading,
    error,
  }

  return <BatchImagesContext.Provider value={value}>{children}</BatchImagesContext.Provider>
}

/**
 * Hook to access batched images for a specific media item
 * Must be used within a BatchImagesProvider
 */
// eslint-disable-next-line react-refresh/only-export-components
export const useBatchImages = (mediaId: number): {
  images: Image[]
  isLoading: boolean
  error: Error | null
} => {
  const context = useContext(BatchImagesContext)

  if (!context) {
    throw new Error('useBatchImages must be used within a BatchImagesProvider')
  }

  return {
    images: context.images[mediaId] || [],
    isLoading: context.isLoading,
    error: context.error,
  }
}

/**
 * Hook to optionally access batched images if available
 * Returns null if not within a BatchImagesProvider
 * Use this when you want to fall back to individual queries
 */
// eslint-disable-next-line react-refresh/only-export-components
export const useBatchImagesIfAvailable = (mediaId: number): {
  images: Image[]
  isLoading: boolean
  error: Error | null
} | null => {
  const context = useContext(BatchImagesContext)

  if (!context) {
    return null
  }

  return {
    images: context.images[mediaId] || [],
    isLoading: context.isLoading,
    error: context.error,
  }
}
