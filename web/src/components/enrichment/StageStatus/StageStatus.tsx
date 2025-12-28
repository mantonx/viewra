import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { useStageStatus, type StageStatus as StageStatusType } from '@/lib/hooks/useStageStatus'
import type { StageStatusProps } from './StageStatus.types'

/** Format stage name for display (e.g., "tmdb" -> "TMDB", "local-images" -> "Local Images") */
const formatStageName = (stage: string): string => {
  // Handle known acronyms
  const acronyms: Record<string, string> = {
    tmdb: 'TMDB',
    ai: 'AI',
  }

  if (acronyms[stage.toLowerCase()]) {
    return acronyms[stage.toLowerCase()]
  }

  return stage
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

/** Get badge color based on circuit state */
const getStateColor = (state: StageStatusType['state']) => {
  switch (state) {
    case 'open':
      return 'red'
    case 'half_open':
      return 'yellow'
    case 'closed':
      return 'green'
    default:
      return 'gray'
  }
}

/** Format state for display */
const formatState = (state: StageStatusType['state']) => {
  switch (state) {
    case 'open':
      return 'Open'
    case 'half_open':
      return 'Recovering'
    case 'closed':
      return 'Healthy'
    default:
      return state
  }
}

/** Countdown timer component */
const RetryCountdown = ({ retryAt }: { retryAt: Date }) => {
  const [remaining, setRemaining] = useState<number>(0)

  useEffect(() => {
    const update = () => {
      const now = Date.now()
      const target = retryAt.getTime()
      const diff = Math.max(0, Math.ceil((target - now) / 1000))
      setRemaining(diff)
    }

    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [retryAt])

  if (remaining <= 0) {
    return <span className="text-zinc-500 dark:text-zinc-400">Retrying soon...</span>
  }

  const minutes = Math.floor(remaining / 60)
  const seconds = remaining % 60

  return (
    <span className="text-zinc-500 dark:text-zinc-400">
      Retry in {minutes > 0 ? `${minutes}m ` : ''}{seconds}s
    </span>
  )
}

/** Single stage status card */
const StageCard = ({
  status,
  onReset,
  isResetting,
}: {
  status: StageStatusType
  onReset: (stage: string) => void
  isResetting: boolean
}) => {
  const isOpen = status.state === 'open'
  const isHalfOpen = status.state === 'half_open'

  return (
    <div
      className={cn(
        'rounded-lg border p-3',
        isOpen && 'border-red-200 bg-red-50 dark:border-red-900/50 dark:bg-red-950/20',
        isHalfOpen && 'border-yellow-200 bg-yellow-50 dark:border-yellow-900/50 dark:bg-yellow-950/20',
        !isOpen && !isHalfOpen && 'border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900/50'
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="font-medium text-zinc-900 dark:text-zinc-100">
            {formatStageName(status.stage)}
          </span>
          <Badge color={getStateColor(status.state)} size="sm">
            {formatState(status.state)}
          </Badge>
        </div>

        {(isOpen || isHalfOpen) && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onReset(status.stage)}
            isLoading={isResetting}
            className="text-xs"
          >
            Reset
          </Button>
        )}
      </div>

      {isOpen && (
        <div className="mt-2 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-red-600 dark:text-red-400">
              {status.consecutiveFailures} consecutive failures
            </span>
            {status.retryAt && <RetryCountdown retryAt={status.retryAt} />}
          </div>
        </div>
      )}

      {isHalfOpen && (
        <div className="mt-2 text-sm text-yellow-600 dark:text-yellow-400">
          Testing recovery...
        </div>
      )}
    </div>
  )
}

/**
 * Displays enrichment stage circuit breaker statuses.
 *
 * Shows which enrichment stages (TMDB, local-images, etc.) are healthy or
 * experiencing issues. When a circuit breaker is open, it displays a countdown
 * until the next retry and allows manual reset.
 *
 * @example
 * ```tsx
 * // Show only problem stages (default behavior)
 * <StageStatus />
 *
 * // Show all stages
 * <StageStatus showOnlyProblems={false} />
 * ```
 */
const StageStatus = ({
  showOnlyProblems = true,
  enabled = true,
  className,
}: StageStatusProps) => {
  const {
    stages,
    problemStages,
    isLoading,
    resetStage,
    isResetting,
    resettingStage,
  } = useStageStatus({ enabled })

  const displayStages = showOnlyProblems ? problemStages : stages

  // Don't render anything if no stages to show
  if (isLoading || displayStages.length === 0) {
    return null
  }

  return (
    <div className={cn('space-y-2', className)}>
      {showOnlyProblems && problemStages.length > 0 && (
        <div className="flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
          <svg
            className="h-4 w-4 text-yellow-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <span>Some enrichment providers are experiencing issues</span>
        </div>
      )}

      {displayStages.map((status) => (
        <StageCard
          key={status.stage}
          status={status}
          onReset={resetStage}
          isResetting={isResetting && resettingStage === status.stage}
        />
      ))}
    </div>
  )
}

export type { StageStatusProps } from './StageStatus.types'
export { StageStatus }
