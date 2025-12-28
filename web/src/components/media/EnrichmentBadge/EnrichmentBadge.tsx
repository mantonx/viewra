import { cn } from '@/lib/utils'
import type { EnrichmentBadgeProps } from './EnrichmentBadge.types'

/**
 * Subtle enrichment status badge for media cards.
 *
 * Shows in the top-left corner of media cards to indicate enrichment status:
 * - Spinning icon for pending/processing items
 * - Warning icon for failed items
 * - Hidden when fully enriched
 */
const EnrichmentBadge = ({ status, className }: EnrichmentBadgeProps) => {
  // Don't render for enriched items
  if (status === 'enriched') {
    return null
  }

  const isProcessing = status === 'pending' || status === 'processing'
  const isFailed = status === 'failed'

  return (
    <div
      className={cn(
        'absolute top-1.5 left-1.5 z-20',
        'w-5 h-5 flex items-center justify-center',
        'rounded-full backdrop-blur-sm',
        isProcessing && 'bg-blue-500/80',
        isFailed && 'bg-orange-500/80',
        className
      )}
      title={
        isProcessing
          ? 'Enrichment in progress'
          : 'Enrichment failed - click to retry'
      }
    >
      {isProcessing && (
        <svg
          className="w-3 h-3 text-white animate-spin"
          fill="none"
          viewBox="0 0 24 24"
          aria-label="Enriching"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      )}
      {isFailed && (
        <svg
          className="w-3 h-3 text-white"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-label="Enrichment failed"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      )}
    </div>
  )
}

export type { EnrichmentBadgeProps, EnrichmentStatus } from './EnrichmentBadge.types'
export { EnrichmentBadge }
