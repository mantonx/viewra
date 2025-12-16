import { useState, useCallback, useEffect } from 'react'
import { useSSE, type SSEConnectionState } from './useSSE'
import type { InternalApiHandlersLibraryEnrichmentProgressResponse } from '@/lib/api/generated/models'

/**
 * Enrichment progress state with computed fields
 */
export interface EnrichmentProgressState {
  /** Library ID */
  libraryId: number
  /** Total items pending enrichment */
  pending: number
  /** Items currently being processed */
  processing: number
  /** Successfully enriched items */
  completed: number
  /** Items that failed enrichment */
  failed: number
  /** Whether enrichment is active (pending or processing > 0) */
  isActive: boolean
  /** Total items that need enrichment (pending + processing + completed + failed) */
  total: number
  /** Progress as percentage (0-100) */
  progressPercent: number
  /** Per-stage breakdown if available */
  stageProgress?: InternalApiHandlersLibraryEnrichmentProgressResponse['stage_progress']
}

export interface UseEnrichmentProgressOptions {
  /** Enable/disable the SSE connection */
  enabled?: boolean
  /** Called when progress updates */
  onProgress?: (state: EnrichmentProgressState) => void
  /** Called when enrichment completes (all items done) */
  onComplete?: () => void
}

export interface UseEnrichmentProgressReturn {
  /** Current enrichment progress state */
  progress: EnrichmentProgressState | null
  /** SSE connection state */
  connectionState: SSEConnectionState
  /** Any connection error */
  error: Error | null
  /** Whether enrichment is active */
  isActive: boolean
  /** Manually reconnect SSE */
  reconnect: () => void
}

/**
 * Hook to stream enrichment progress for a library via SSE.
 *
 * Provides real-time updates on enrichment status including pending,
 * processing, completed, and failed counts.
 *
 * @example
 * ```tsx
 * const { progress, isActive, connectionState } = useEnrichmentProgress(libraryId, {
 *   enabled: true,
 *   onComplete: () => toast.success('Enrichment complete!'),
 * })
 *
 * if (isActive && progress) {
 *   return <div>Enriching: {progress.completed}/{progress.total}</div>
 * }
 * ```
 */
export const useEnrichmentProgress = (
  libraryId: number,
  options: UseEnrichmentProgressOptions = {}
): UseEnrichmentProgressReturn => {
  const { enabled = true, onProgress, onComplete } = options

  const [progress, setProgress] = useState<EnrichmentProgressState | null>(null)
  const [wasActive, setWasActive] = useState(false)

  const handleEvent = useCallback(
    (data: InternalApiHandlersLibraryEnrichmentProgressResponse) => {
      const pending = data.total_pending ?? 0
      const processing = data.total_processing ?? 0
      const completed = data.total_completed ?? 0
      const failed = data.total_failed ?? 0
      const total = pending + processing + completed + failed
      const isActive = data.is_active ?? (pending > 0 || processing > 0)

      const state: EnrichmentProgressState = {
        libraryId,
        pending,
        processing,
        completed,
        failed,
        isActive,
        total,
        progressPercent: total > 0 ? Math.round((completed / total) * 100) : 0,
        stageProgress: data.stage_progress,
      }

      setProgress(state)
      onProgress?.(state)

      // Track completion transition
      if (wasActive && !isActive && total > 0) {
        onComplete?.()
      }
      setWasActive(isActive)
    },
    [libraryId, onProgress, onComplete, wasActive]
  )

  const {
    connectionState,
    error,
    reconnect,
  } = useSSE<InternalApiHandlersLibraryEnrichmentProgressResponse>(
    `/api/libraries/${libraryId}/enrichment/stream`,
    {
      enabled: enabled && libraryId > 0,
      onEvent: handleEvent,
      eventTypes: ['init', 'update'],
      reconnectDelay: 5000,
      maxReconnectAttempts: 10,
    }
  )

  // Reset progress when disabled or library changes
  useEffect(() => {
    if (!enabled || libraryId <= 0) {
      setProgress(null)
      setWasActive(false)
    }
  }, [enabled, libraryId])

  return {
    progress,
    connectionState,
    error,
    isActive: progress?.isActive ?? false,
    reconnect,
  }
}

export default useEnrichmentProgress
