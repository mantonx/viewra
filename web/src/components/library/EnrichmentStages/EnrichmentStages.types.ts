import type { GithubComMantonxViewraInternalDomainEnrichmentQueueStats } from '@/lib/api/generated/models'
import type { GithubComMantonxViewraInternalApplicationEnrichmentPipelineCircuitBreakerStatus } from '@/lib/api/generated/models'

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
  circuitStatus?: GithubComMantonxViewraInternalApplicationEnrichmentPipelineCircuitBreakerStatus
}

export interface EnrichmentStagesProps {
  /** Stage progress data from the enrichment progress hook */
  stageProgress?: Record<string, GithubComMantonxViewraInternalDomainEnrichmentQueueStats>
  /** Circuit breaker statuses (optional, fetched separately) */
  circuitStatuses?: GithubComMantonxViewraInternalApplicationEnrichmentPipelineCircuitBreakerStatus[]
  /** Whether to show circuit breaker warnings */
  showCircuitStatus?: boolean
  /** Additional CSS classes */
  className?: string
}
