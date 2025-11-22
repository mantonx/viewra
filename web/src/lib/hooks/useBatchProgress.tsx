/**
 * useBatchProgress Hook & Context
 * Provides efficient batch progress loading to eliminate N+1 queries
 *
 * Usage:
 * 1. Wrap components with: <BatchProgressProvider mediaIds={[1,2,3,...]}>
 * 2. Use useBatchProgress(mediaId) in child components to get progress
 */

import type { ReactNode } from 'react'
import { createContext, useContext, useMemo } from 'react'
import { useQueries } from '@tanstack/react-query'
import { progressApi } from '@/lib/api'
import type { GithubComViewraViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse } from '@/lib/api/generated/models'

interface BatchProgressContextValue {
  progress: Record<number, WatchProgressResponse>
  isLoading: boolean
  error: Error | null
}

const BatchProgressContext = createContext<BatchProgressContextValue | null>(null)

interface BatchProgressProviderProps {
  mediaIds: number[]
  children: ReactNode
}

/**
 * Provider component that fetches progress for multiple media items in batched chunks
 * Handles infinite scroll by splitting IDs into 50-item batches and merging results
 */
export const BatchProgressProvider = ({ mediaIds, children }: BatchProgressProviderProps) => {
  // Split IDs into chunks of 50 to avoid overwhelming the backend
  const BATCH_SIZE = 50
  const batches = useMemo(() => {
    const chunks: number[][] = []
    for (let i = 0; i < mediaIds.length; i += BATCH_SIZE) {
      chunks.push(mediaIds.slice(i, i + BATCH_SIZE))
    }
    return chunks
  }, [mediaIds]) // Only recalculate when the actual IDs change

  // Use useQueries to fetch all batches in parallel and cache them independently
  const queries = useQueries({
    queries: batches.map((batch) => ({
      queryKey: ['batch-progress', batch.sort().join(',')],
      queryFn: async () => {
        if (batch.length === 0) {
          return { progress: {} }
        }

        const response = await progressApi.getBatchProgress(batch)
        // customInstance returns { data, status, headers }, extract the data
        return (response as unknown as { data: { progress: Record<string, WatchProgressResponse> } }).data
      },
      staleTime: 1000 * 30, // 30 seconds - progress changes more frequently than images
    })),
  })

  // Merge all batch results into a single progress map
  const mergedProgress = useMemo(() => {
    const result: Record<number, WatchProgressResponse> = {}
    queries.forEach(query => {
      if (query.data?.progress) {
        // Convert string keys to numbers and merge
        Object.entries(query.data.progress).forEach(([mediaIdStr, progressData]) => {
          const mediaId = parseInt(mediaIdStr, 10)
          if (progressData) {
            result[mediaId] = progressData
          }
        })
      }
    })
    return result
  }, [queries])

  const isLoading = queries.some(q => q.isLoading)
  const error = queries.find(q => q.error)?.error as Error | null

  const value: BatchProgressContextValue = {
    progress: mergedProgress,
    isLoading,
    error,
  }

  return <BatchProgressContext.Provider value={value}>{children}</BatchProgressContext.Provider>
}

/**
 * Hook to access batched progress for a specific media item
 * Must be used within a BatchProgressProvider
 */
// eslint-disable-next-line react-refresh/only-export-components
export const useBatchProgress = (mediaId: number): {
  progress: WatchProgressResponse | null
  isLoading: boolean
  error: Error | null
} => {
  const context = useContext(BatchProgressContext)

  if (!context) {
    throw new Error('useBatchProgress must be used within a BatchProgressProvider')
  }

  return {
    progress: context.progress[mediaId] || null,
    isLoading: context.isLoading,
    error: context.error,
  }
}

/**
 * Hook to optionally access batched progress if available
 * Returns null if not within a BatchProgressProvider
 * Use this when you want to fall back to individual queries
 */
// eslint-disable-next-line react-refresh/only-export-components
export const useBatchProgressIfAvailable = (mediaId: number): {
  progress: WatchProgressResponse | null
  isLoading: boolean
  error: Error | null
} | null => {
  const context = useContext(BatchProgressContext)

  if (!context) {
    return null
  }

  return {
    progress: context.progress[mediaId] || null,
    isLoading: context.isLoading,
    error: context.error,
  }
}
