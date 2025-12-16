import { Sparkles } from 'lucide-react'
import { useEnrichmentProgress } from '@/lib/hooks'
import type { EnrichmentIndicatorProps } from './EnrichmentIndicator.types'

/**
 * Displays enrichment progress for a library.
 * Shows a compact inline indicator when enrichment is active.
 *
 * During active scans, this shows alongside scan progress.
 * When scan is complete, shows as standalone progress indicator.
 *
 * Uses SSE for real-time updates - no polling overhead.
 */
const EnrichmentIndicator = ({
  libraryId,
  enabled = true,
  onComplete,
  compact = false,
}: EnrichmentIndicatorProps) => {
  const { progress, isActive, connectionState } = useEnrichmentProgress(libraryId, {
    enabled,
    onComplete,
  })

  // Don't render if not active or no data
  if (!isActive || !progress) {
    return null
  }

  const { pending, processing, completed, failed } = progress
  const inProgress = pending + processing

  // Compact mode: just show inline text (for use alongside scan progress)
  if (compact) {
    return (
      <span className="inline-flex items-center gap-1.5 text-sm text-amber-600 dark:text-amber-400">
        <Sparkles className="w-3.5 h-3.5 animate-pulse" aria-hidden="true" />
        <span>
          {inProgress > 0 && `${inProgress.toLocaleString()} enriching`}
          {inProgress > 0 && completed > 0 && ', '}
          {completed > 0 && `${completed.toLocaleString()} enriched`}
          {failed > 0 && (
            <span className="text-red-500 dark:text-red-400">
              {' '}({failed.toLocaleString()} failed)
            </span>
          )}
        </span>
        {connectionState === 'error' && (
          <span className="text-xs text-red-500 dark:text-red-400">(disconnected)</span>
        )}
      </span>
    )
  }

  // Full mode: show as a standalone section
  return (
    <div className="px-4 pb-4">
      <div className="flex items-center gap-2">
        <Sparkles
          className="w-4 h-4 text-amber-500 dark:text-amber-400 animate-pulse"
          aria-hidden="true"
        />
        <span className="text-sm text-amber-700 dark:text-amber-300">
          <span className="font-medium">Enriching metadata:</span>
          {' '}
          {inProgress > 0 && `${inProgress.toLocaleString()} queued`}
          {inProgress > 0 && completed > 0 && ', '}
          {completed > 0 && `${completed.toLocaleString()} done`}
          {failed > 0 && (
            <span className="text-red-600 dark:text-red-400">
              {' '}({failed.toLocaleString()} failed)
            </span>
          )}
        </span>
        {connectionState === 'error' && (
          <span className="text-xs text-red-500 dark:text-red-400 ml-1">(disconnected)</span>
        )}
      </div>
    </div>
  )
}

export type { EnrichmentIndicatorProps } from './EnrichmentIndicator.types'
export { EnrichmentIndicator }
