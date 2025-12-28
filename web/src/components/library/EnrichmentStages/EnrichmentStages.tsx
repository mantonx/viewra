import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import type { EnrichmentStagesProps, StageProgress } from './EnrichmentStages.types'
import type { GithubComMantonxViewraInternalDomainEnrichmentQueueStats } from '@/lib/api/generated/models'

/** Format stage name for display (e.g., "nfo-files" -> "NFO Files", "tmdb" -> "TMDB") */
const formatStageName = (stage: string): string => {
  // Handle special cases
  const specialCases: Record<string, string> = {
    'nfo-files': 'NFO Files',
    'tmdb': 'TMDB',
    'musicbrainz': 'MusicBrainz',
  }

  if (specialCases[stage.toLowerCase()]) {
    return specialCases[stage.toLowerCase()]
  }

  return stage
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

/** Convert raw stage stats to processed StageProgress */
const processStageProgress = (
  name: string,
  stats: GithubComMantonxViewraInternalDomainEnrichmentQueueStats
): StageProgress => {
  const completed = stats.completedCount ?? 0
  const pending = stats.pendingCount ?? 0
  const processing = stats.processingCount ?? 0
  const failed = stats.failedCount ?? 0
  const skipped = stats.skippedCount ?? 0
  const total = stats.totalCount ?? (completed + pending + processing + failed + skipped)

  return {
    name,
    completed,
    total,
    pending,
    processing,
    failed,
    skipped,
    isActive: processing > 0,
    isComplete: total > 0 && (completed + skipped + failed) >= total,
  }
}

/**
 * Displays per-stage enrichment progress breakdown.
 *
 * Shows each stage with its status icon and progress counts.
 * Optionally displays circuit breaker warnings for stages with open circuits.
 */
const EnrichmentStages = ({
  stageProgress,
  circuitStatuses,
  showCircuitStatus = true,
  className,
}: EnrichmentStagesProps) => {
  if (!stageProgress || Object.keys(stageProgress).length === 0) {
    return null
  }

  // Convert to array and sort by stage order (if available) or alphabetically
  const stages: StageProgress[] = Object.entries(stageProgress)
    .map(([name, stats]) => processStageProgress(name, stats))
    .sort((a, b) => a.name.localeCompare(b.name))

  // Create a map of circuit statuses by stage name
  const circuitMap = new Map(
    circuitStatuses?.map((status) => [status.stage, status]) ?? []
  )

  return (
    <div className={cn('space-y-1.5', className)}>
      <p className={cn('text-xs font-medium uppercase tracking-wide mb-2', text.tertiary)}>
        Stages
      </p>
      {stages.map((stage) => {
        const circuit = circuitMap.get(stage.name)
        const isCircuitOpen = showCircuitStatus && circuit?.state === 'open'

        return (
          <div
            key={stage.name}
            className="flex items-center justify-between text-sm"
          >
            <div className="flex items-center gap-2">
              {/* Status icon */}
              {stage.isComplete ? (
                <svg
                  className="w-4 h-4 text-green-500"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-label="Completed"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              ) : stage.isActive ? (
                <svg
                  className="w-4 h-4 text-blue-500 animate-spin"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-label="In progress"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
              ) : (
                <svg
                  className={cn('w-4 h-4', text.tertiary)}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-label="Pending"
                >
                  <circle cx="12" cy="12" r="9" strokeWidth={2} />
                </svg>
              )}

              {/* Stage name */}
              <span className={cn(stage.isComplete ? text.secondary : text.primary)}>
                {formatStageName(stage.name)}
              </span>

              {/* Circuit breaker warning */}
              {isCircuitOpen && (
                <span
                  className="text-yellow-500"
                  title="Circuit breaker open - stage temporarily paused"
                >
                  <svg
                    className="w-4 h-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                </span>
              )}
            </div>

            {/* Progress count */}
            <div className="flex items-center gap-2">
              <span className={cn('font-mono text-xs', text.tertiary)}>
                {stage.completed.toLocaleString()}/{stage.total.toLocaleString()}
              </span>
              {stage.failed > 0 && (
                <span className="text-xs text-red-500">
                  ({stage.failed} failed)
                </span>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

export type { EnrichmentStagesProps, StageProgress } from './EnrichmentStages.types'
export { EnrichmentStages }
