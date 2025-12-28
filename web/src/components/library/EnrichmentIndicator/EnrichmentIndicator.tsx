import { useEnrichmentProgress } from '@/lib/hooks'
import { Progress } from '@/components/ui'
import { EnrichmentStages } from '@/components/library/EnrichmentStages'
import type { EnrichmentIndicatorProps } from './EnrichmentIndicator.types'

/** Format stage name for display (e.g., "local-images" -> "Local Images") */
const formatStageName = (stage: string): string => {
  return stage
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

/**
 * Displays enrichment progress for a library.
 * Shows current stage, item being enriched, and progress count.
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

  const { currentItem, failed, overallProgress, stageProgress } = progress

  // Use overall progress for accurate percentage (unique items, not inflated by stages)
  const progressPercent = overallProgress?.percentage ?? 0
  const completedItems = overallProgress?.completedItems ?? 0
  const totalItems = overallProgress?.totalItems ?? 0

  // Build the status text: "stage: title"
  const statusParts: string[] = []
  if (currentItem?.stage) {
    statusParts.push(formatStageName(currentItem.stage))
  }
  if (currentItem?.title) {
    statusParts.push(currentItem.title)
  }

  const statusText = statusParts.length > 0
    ? statusParts.join(': ')
    : 'Enriching...'

  // Compact mode: just show inline text (for use alongside scan progress)
  if (compact) {
    return (
      <span className="text-sm text-zinc-600 dark:text-zinc-400">
        {statusText} ({completedItems.toLocaleString()}/{totalItems.toLocaleString()})
        {failed > 0 && (
          <span className="text-red-500 dark:text-red-400">
            {' '}• {failed.toLocaleString()} failed
          </span>
        )}
        {connectionState === 'error' && (
          <span className="text-red-500 dark:text-red-400"> (disconnected)</span>
        )}
      </span>
    )
  }

  // Full mode: show progress bar with enrichment status
  const label = `Enriching: ${statusText} (${completedItems.toLocaleString()}/${totalItems.toLocaleString()})`

  // Check if we have meaningful stage data to show
  const hasStageData = stageProgress && Object.keys(stageProgress).length > 0

  return (
    <div className="px-4 pb-4 space-y-3">
      <Progress
        value={progressPercent}
        label={label}
        variant="default"
        size="sm"
        showPercentage
      />
      {hasStageData && (
        <EnrichmentStages
          stageProgress={stageProgress}
          showCircuitStatus
        />
      )}
      {failed > 0 && (
        <p className="text-xs text-red-500 dark:text-red-400">
          {failed.toLocaleString()} failed
        </p>
      )}
      {connectionState === 'error' && (
        <p className="text-xs text-red-500 dark:text-red-400">
          Connection lost - reconnecting...
        </p>
      )}
    </div>
  )
}

export type { EnrichmentIndicatorProps } from './EnrichmentIndicator.types'
export { EnrichmentIndicator }
