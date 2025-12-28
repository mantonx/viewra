import { useCallback, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  useGetApiLibrariesIdEnrichmentFailures,
  postApiLibrariesIdEnrichmentFailuresRetry,
  postApiEnrichmentRetry,
  getGetApiLibrariesIdEnrichmentFailuresQueryKey,
} from '@/lib/api/generated/enrichment/enrichment'
import type { InternalApiHandlersEnrichmentFailureResponse } from '@/lib/api/generated/models'

export interface EnrichmentFailure {
  id: number
  mediaId: number
  mediaType: string
  title: string
  stage: string
  attempts: number
  maxAttempts: number
  errorMessage: string
  errorCategory: string
  lastAttemptAt: string
}

const mapFailure = (raw: InternalApiHandlersEnrichmentFailureResponse): EnrichmentFailure => ({
  id: raw.id ?? 0,
  mediaId: raw.media_id ?? 0,
  mediaType: raw.media_type ?? '',
  title: raw.title ?? '',
  stage: raw.stage ?? '',
  attempts: raw.attempts ?? 0,
  maxAttempts: raw.max_attempts ?? 0,
  errorMessage: raw.error_message ?? '',
  errorCategory: raw.error_category ?? '',
  lastAttemptAt: raw.last_attempt_at ?? '',
})

interface UseEnrichmentFailuresOptions {
  /** Library ID to fetch failures for */
  libraryId: number
  /** Enable/disable fetching (default: true) */
  enabled?: boolean
  /** Maximum number of failures to fetch (default: 50) */
  limit?: number
  /** Offset for pagination (default: 0) */
  offset?: number
}

/**
 * Hook to fetch and manage enrichment failures for a library.
 *
 * @example
 * ```tsx
 * const { failures, total, isLoading, retryAll, isRetrying } = useEnrichmentFailures({
 *   libraryId: 123,
 * })
 * ```
 */
export const useEnrichmentFailures = ({
  libraryId,
  enabled = true,
  limit = 50,
  offset = 0,
}: UseEnrichmentFailuresOptions) => {
  const queryClient = useQueryClient()
  const [retryingIds, setRetryingIds] = useState<Set<number>>(new Set())

  const { data, isLoading, error, refetch } = useGetApiLibrariesIdEnrichmentFailures(
    libraryId,
    { limit, offset },
    {
      query: {
        enabled: enabled && libraryId > 0,
        staleTime: 30000, // 30 seconds
      },
    }
  )

  const retryAllMutation = useMutation({
    mutationFn: () => postApiLibrariesIdEnrichmentFailuresRetry(libraryId),
    onSuccess: () => {
      // Invalidate failures query to refetch
      queryClient.invalidateQueries({
        queryKey: getGetApiLibrariesIdEnrichmentFailuresQueryKey(libraryId),
      })
    },
  })

  const retrySingleMutation = useMutation({
    mutationFn: (jobId: number) => postApiEnrichmentRetry({ job_id: jobId }),
    onMutate: (jobId) => {
      setRetryingIds((prev) => new Set(prev).add(jobId))
    },
    onSettled: (_, __, jobId) => {
      setRetryingIds((prev) => {
        const next = new Set(prev)
        next.delete(jobId)
        return next
      })
    },
    onSuccess: () => {
      // Invalidate failures query to refetch
      queryClient.invalidateQueries({
        queryKey: getGetApiLibrariesIdEnrichmentFailuresQueryKey(libraryId),
      })
    },
  })

  const failures: EnrichmentFailure[] =
    data?.status === 200 ? (data.data.failures ?? []).map(mapFailure) : []
  const total = data?.status === 200 ? data.data.total ?? 0 : 0

  const retryAll = useCallback(() => {
    retryAllMutation.mutate()
  }, [retryAllMutation])

  const retrySingle = useCallback(
    (jobId: number) => {
      retrySingleMutation.mutate(jobId)
    },
    [retrySingleMutation]
  )

  const isRetryingSingle = useCallback(
    (jobId: number) => retryingIds.has(jobId),
    [retryingIds]
  )

  return {
    /** List of enrichment failures */
    failures,
    /** Total count of failures (for pagination) */
    total,
    /** Whether failures are being fetched */
    isLoading,
    /** Any error from fetching */
    error,
    /** Retry all failed jobs */
    retryAll,
    /** Whether retry-all is in progress */
    isRetrying: retryAllMutation.isPending,
    /** Retry a single failed job by ID */
    retrySingle,
    /** Check if a specific job is being retried */
    isRetryingSingle,
    /** Refetch failures */
    refetch,
  }
}

export default useEnrichmentFailures
