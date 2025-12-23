import { Progress } from '@/components/ui'
import { formatETA } from '@/lib/utils/format'
import type { ScanProgressState } from '@/lib/hooks/useScanProgress'
import type { EnrichmentProgressState } from '@/lib/hooks/useEnrichmentProgress'

export interface LibraryProgressProps {
  /** Current scan progress state */
  scanStatus: ScanProgressState | null
  /** Current enrichment progress state */
  enrichmentProgress: EnrichmentProgressState | null
  /** Whether scan is actively running */
  isScanning: boolean
  /** Whether scan is paused */
  isPaused: boolean
  /** Whether enrichment is active */
  isEnrichmentActive: boolean
}

/**
 * Progress display for library scanning and enrichment.
 *
 * Two distinct phases:
 * 1. During scan: Shows scan progress (files X/Y) with enrichment activity below
 * 2. After scan: Shows enrichment progress (X% complete, Y remaining)
 *
 * This separation exists because during scanning, the total items to enrich
 * keeps growing. Only after scan completes can we show meaningful enrichment %.
 */
const LibraryProgress = ({
  scanStatus,
  enrichmentProgress,
  isScanning,
  isPaused,
  isEnrichmentActive,
}: LibraryProgressProps) => {
  // Determine current phase
  const isDiscovering =
    (isScanning || isPaused) && scanStatus?.phase === 'discovering' && !scanStatus.discoveryDone
  const isScanningFiles = (isScanning || isPaused) && !isDiscovering
  const scanComplete = scanStatus?.isCompleted && !isScanning && !isPaused
  const isEnrichmentOnlyPhase = scanComplete && isEnrichmentActive

  // Don't render if nothing is happening
  if (!isScanning && !isPaused && !isEnrichmentActive) {
    return null
  }
  // Don't render if everything is done
  if (scanComplete && !isEnrichmentActive) {
    return null
  }

  // Truncate text to max length with ellipsis
  const truncate = (text: string, maxLength: number): string => {
    if (text.length <= maxLength) {return text}
    return `${text.slice(0, maxLength - 1)  }...`
  }

  // Get current enrichment item display
  const getCurrentEnrichmentDisplay = (): string | null => {
    if (!isEnrichmentActive || !enrichmentProgress) {return null}

    const currentItem = enrichmentProgress.currentItem
    if (currentItem) {
      const displayName = currentItem.title || ''
      const stageName = currentItem.stage || 'Enriching'

      if (displayName) {
        return `${stageName}: ${truncate(displayName, 40)}`
      }
      return `${stageName}...`
    }

    return 'Enriching...'
  }

  // Calculate enrichment progress (only meaningful after scan completes)
  const getEnrichmentProgress = () => {
    if (!enrichmentProgress) {return { percent: 0, remaining: 0, total: 0 }}

    const { completed, pending, processing, failed, total } = enrichmentProgress
    const remaining = pending + processing
    // Progress = completed / total (where total = completed + pending + processing + failed)
    const percent = total > 0 ? Math.round((completed / total) * 100) : 0

    return { percent, remaining, total, completed, failed }
  }

  // PHASE 1: Scanning in progress
  if (isDiscovering || isScanningFiles) {
    const scanProgress = scanStatus?.progress ?? 0
    const eta = scanStatus?.etaSeconds ? formatETA(scanStatus.etaSeconds) : null
    const enrichmentDisplay = getCurrentEnrichmentDisplay()

    let statusText: string
    if (isPaused) {
      statusText = 'Paused'
    } else if (isDiscovering) {
      const found = scanStatus?.filesFound ?? 0
      const estimated = scanStatus?.estimatedTotal ?? 0
      statusText = `Discovering files... ${found.toLocaleString()} found${
        estimated > 0 ? ` (est. ${estimated.toLocaleString()})` : ''
      }`
    } else {
      const processed = scanStatus?.filesProcessed ?? 0
      const found = scanStatus?.filesFound ?? 0
      statusText = `Scanning ${processed.toLocaleString()} / ${found.toLocaleString()} files`
    }

    return (
      <div className="px-4 pb-4 space-y-2">
        <div className="space-y-1">
          <div className="flex items-center justify-between text-sm">
            <span className="text-neutral-700 dark:text-neutral-300">{statusText}</span>
            <span className="text-neutral-600 dark:text-neutral-400">
              {!isDiscovering && `${Math.round(scanProgress)}%`}
              {eta && ` - ETA ${eta}`}
            </span>
          </div>
          <Progress
            value={isDiscovering ? 0 : scanProgress}
            variant={isPaused ? 'warning' : 'default'}
            size="sm"
          />
        </div>

        {/* Show enrichment activity below scan progress */}
        {enrichmentDisplay && (
          <div className="text-xs text-neutral-600 dark:text-neutral-400">{enrichmentDisplay}</div>
        )}
      </div>
    )
  }

  // PHASE 2: Scan complete, enrichment in progress
  if (isEnrichmentOnlyPhase) {
    const { percent, remaining } = getEnrichmentProgress()
    const enrichmentDisplay = getCurrentEnrichmentDisplay()

    return (
      <div className="px-4 pb-4 space-y-2">
        <div className="space-y-1">
          <div className="flex items-center justify-between text-sm">
            <span className="text-neutral-700 dark:text-neutral-300">
              {enrichmentDisplay || 'Enriching...'}
            </span>
            <span className="text-neutral-600 dark:text-neutral-400">
              {percent}%{remaining > 0 && ` - ${remaining.toLocaleString()} remaining`}
            </span>
          </div>
          <Progress value={percent} size="sm" />
        </div>
      </div>
    )
  }

  // Fallback (shouldn't reach here, but just in case)
  return null
}

export { LibraryProgress }
