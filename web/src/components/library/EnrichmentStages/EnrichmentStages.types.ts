import type { GithubComMantonxViewraInternalDomainEnrichmentQueueStats } from '@/lib/api/generated/models'

/**
 * Simplified circuit breaker status interface that works with both
 * the full pipeline type and the handler response type.
 */
export interface CircuitBreakerStatusInfo {
  stage?: string
  state?: string
  consecutive_failures?: number
  consecutiveFailures?: number
  failure_threshold?: number
  failureThreshold?: number
}

export interface StageProgress {
  /** Stage name (e.g., "nfo-files", "tmdb", "local-images") */
  name: string
  /** Number of completed items */
  completed: number
  /** Total number of items for this stage */
  total: number
  /** Number of pending items */
  pending: number
  /** Number of processing items */
  processing: number
  /** Number of failed items */
  failed: number
  /** Number of skipped items */
  skipped: number
  /** Whether this stage is currently active (processing > 0) */
  isActive: boolean
  /** Whether this stage is complete (completed + skipped + failed === total) */
  isComplete: boolean
  /** Circuit breaker status if available */
  circuitStatus?: CircuitBreakerStatusInfo
}

export interface EnrichmentStagesProps {
  /** Stage progress data from the enrichment progress hook */
  stageProgress?: Record<string, GithubComMantonxViewraInternalDomainEnrichmentQueueStats>
  /** Circuit breaker statuses (optional, fetched separately) */
  circuitStatuses?: CircuitBreakerStatusInfo[]
  /** Whether to show circuit breaker warnings */
  showCircuitStatus?: boolean
  /** Additional CSS classes */
  className?: string
}
