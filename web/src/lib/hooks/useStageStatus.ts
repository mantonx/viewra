import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  useGetApiEnrichmentStages,
  getGetApiEnrichmentStagesQueryKey,
  postApiEnrichmentStagesStageReset,
} from '@/lib/api/generated/enrichment/enrichment'
import type { GithubComMantonxViewraInternalApplicationEnrichmentPipelineCircuitBreakerStatus } from '@/lib/api/generated/models'

type CircuitState = 'closed' | 'open' | 'half_open'

export interface StageStatus {
  stage: string
  state: CircuitState
  consecutiveFailures: number
  failureThreshold: number
  lastStateChange: Date | null
  lastFailure: Date | null
  retryAfterSeconds: number | null
  retryAt: Date | null
}

const mapStatus = (
  raw: GithubComMantonxViewraInternalApplicationEnrichmentPipelineCircuitBreakerStatus
): StageStatus => ({
  stage: raw.stage ?? '',
  state: (raw.state as CircuitState) ?? 'closed',
  consecutiveFailures: raw.consecutive_failures ?? 0,
  failureThreshold: raw.failure_threshold ?? 10,
  lastStateChange: raw.last_state_change ? new Date(raw.last_state_change) : null,
  lastFailure: raw.last_failure ? new Date(raw.last_failure) : null,
  // retry_after_seconds comes as nanoseconds from Go's time.Duration
  retryAfterSeconds: raw.retry_after_seconds ? Math.ceil(raw.retry_after_seconds / 1_000_000_000) : null,
  retryAt: raw.retry_at ? new Date(raw.retry_at) : null,
})

interface UseStageStatusOptions {
  /** Enable polling for status updates (default: true) */
  enabled?: boolean
  /** Polling interval in ms (default: 10000) */
  refetchInterval?: number
}

/**
 * Hook to fetch and manage enrichment stage circuit breaker statuses.
 *
 * @example
 * ```tsx
 * const { stages, isLoading, resetStage, isResetting } = useStageStatus()
 *
 * // Display open circuits
 * const openCircuits = stages.filter(s => s.state === 'open')
 *
 * // Reset a circuit breaker
 * resetStage('tmdb')
 * ```
 */
export const useStageStatus = (options: UseStageStatusOptions = {}) => {
  const { enabled = true, refetchInterval = 10000 } = options

  const queryClient = useQueryClient()

  const { data, isLoading, error } = useGetApiEnrichmentStages({
    query: {
      enabled,
      refetchInterval,
      staleTime: 5000,
    },
  })

  const resetMutation = useMutation({
    mutationFn: (stage: string) => postApiEnrichmentStagesStageReset(stage),
    onSuccess: () => {
      // Invalidate to refetch statuses
      queryClient.invalidateQueries({ queryKey: getGetApiEnrichmentStagesQueryKey() })
    },
  })

  const stages: StageStatus[] = data?.data?.map(mapStatus) ?? []

  const resetStage = useCallback(
    (stage: string) => {
      resetMutation.mutate(stage)
    },
    [resetMutation]
  )

  // Filter to only stages with issues (open or half-open)
  const problemStages = stages.filter((s) => s.state !== 'closed')

  return {
    /** All stage statuses */
    stages,
    /** Stages with open or half-open circuits */
    problemStages,
    /** Whether the initial load is in progress */
    isLoading,
    /** Any error from fetching */
    error,
    /** Reset a circuit breaker for a stage */
    resetStage,
    /** Whether a reset is in progress */
    isResetting: resetMutation.isPending,
    /** The stage currently being reset */
    resettingStage: resetMutation.variables,
  }
}

export default useStageStatus
