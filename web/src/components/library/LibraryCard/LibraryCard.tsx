import { ScanErrorsDialog, type IssueTab } from '@/components/library/ScanErrorsDialog'
import { LibrarySettingsModal } from '@/components/library/LibrarySettingsModal'
import { Button, Progress } from '@/components/ui'
import {
  useDeleteApiLibrariesId,
  usePostApiLibrariesIdScan,
  usePostApiLibrariesIdScanJobIdPause,
  usePostApiLibrariesIdScanJobIdResume,
} from '@/lib/api'
import { useConfirm } from '@/lib/hooks/useConfirm'
import { useEnrichmentProgress } from '@/lib/hooks/useEnrichmentProgress'
import { useEnrichmentFailures } from '@/lib/hooks/useEnrichmentFailures'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useScanProgress } from '@/lib/hooks/useScanProgress'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { formatETA } from '@/lib/utils/format'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import type { LibraryCardProps } from './LibraryCard.types'

/** Format stage name for display */
const formatStageName = (stage: string): string => {
  const specialCases: Record<string, string> = {
    'nfo': 'NFO',
    'tmdb': 'TMDB',
    'musicbrainz': 'MusicBrainz',
    'ai-search': 'AI Search',
    'local-images': 'Local Images',
  }
  return specialCases[stage.toLowerCase()] ?? stage.charAt(0).toUpperCase() + stage.slice(1)
}

const LibraryCard = ({ library }: LibraryCardProps) => {
  const [showErrorsDialog, setShowErrorsDialog] = useState(false)
  const [dialogInitialTab, setDialogInitialTab] = useState<IssueTab | undefined>(undefined)
  const [isExpanded, setIsExpanded] = useState(false)
  const invalidateLibraries = useInvalidateLibraries()
  const [showSettingsModal, setShowSettingsModal] = useState(false)
  const deleteMutation = useDeleteApiLibrariesId()
  const scanMutation = usePostApiLibrariesIdScan()
  const pauseMutation = usePostApiLibrariesIdScanJobIdPause()
  const resumeMutation = usePostApiLibrariesIdScanJobIdResume()
  const toast = useToast()
  const { confirm } = useConfirm()

  const libraryId = library.id ?? 0

  const { scanStatus, isScanning, isPaused } = useScanProgress(libraryId, {
    enabled: libraryId > 0,
  })

  const { isActive: isEnriching, progress: enrichmentProgress } = useEnrichmentProgress(libraryId, {
    enabled: libraryId > 0,
  })

  const enrichmentComplete = enrichmentProgress !== null && !isEnriching

  const { total: enrichmentFailureCount } = useEnrichmentFailures({
    libraryId,
    enabled: libraryId > 0 && !isScanning && enrichmentComplete,
    limit: 1,
  })

  const handleDelete = async () => {
    if (!library.id || !library.name) {return}
    const confirmed = await confirm({
      title: 'Delete Library',
      message: `Are you sure you want to delete "${library.name}"? This action cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })
    if (!confirmed) {return}
    try {
      await deleteMutation.mutateAsync({ id: library.id })
      invalidateLibraries()
      toast.success('Library deleted successfully')
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to delete library'))
    }
  }

  const handleScan = async () => {
    if (!library.id) {return}
    try {
      await scanMutation.mutateAsync({ id: library.id })
      toast.success('Scan started')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to start scan'))
    }
  }

  const handlePause = async () => {
    if (!library.id || !scanStatus?.jobId) {return}
    try {
      await pauseMutation.mutateAsync({ id: library.id, jobId: scanStatus.jobId })
      toast.success('Scan paused')
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to pause scan'))
    }
  }

  const handleResume = async () => {
    if (!library.id || !scanStatus?.jobId) {return}
    try {
      await resumeMutation.mutateAsync({ id: library.id, jobId: scanStatus.jobId })
      toast.success('Scan resumed')
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to resume scan'))
    }
  }

  // Derived state
  const isCompleted = scanStatus?.isCompleted ?? false
  const scanIssueCount = (scanStatus?.errorCount ?? 0) + (scanStatus?.warningCount ?? 0)
  const totalIssueCount = scanIssueCount + enrichmentFailureCount
  const scanEtaDisplay = scanStatus?.etaSeconds ? formatETA(scanStatus.etaSeconds) : null
  const enrichmentEtaDisplay = enrichmentProgress?.etaSeconds ? formatETA(enrichmentProgress.etaSeconds) : null

  // Helper to open issues dialog to a specific tab
  const openIssuesDialog = (tab?: IssueTab) => {
    setDialogInitialTab(tab)
    setShowErrorsDialog(true)
  }

  // Get enrichment stage data
  const stageProgress = enrichmentProgress?.stageProgress
  const stages = stageProgress
    ? Object.entries(stageProgress).map(([name, stats]) => {
        const pending = stats.pendingCount ?? 0
        const processing = stats.processingCount ?? 0
        const total = stats.totalCount ?? 0
        // Stage is complete when there's nothing left to process
        // This is more stable than comparing completed+skipped+failed >= total
        // because total can increase as new items are discovered during scanning
        const isComplete = total > 0 && pending === 0 && processing === 0
        // Stage is active if it has work remaining (pending or processing)
        // Using pending > 0 || processing > 0 prevents flickering because pending
        // stays non-zero even when processing fluctuates between 0 and 1
        const isActive = pending > 0 || processing > 0
        return {
          name,
          completed: stats.completedCount ?? 0,
          total,
          failed: stats.failedCount ?? 0,
          isComplete,
          isActive,
        }
      })
    : []

  // Status summary for collapsed view
  const getStatusSummary = () => {
    if (isScanning) {return 'Scanning...'}
    if (isPaused) {return 'Paused'}
    if (isEnriching) {
      const pct = Math.round(enrichmentProgress?.overallProgress?.percentage ?? 0)
      return `Enriching ${pct}%`
    }
    if (isCompleted && enrichmentComplete) {return 'Complete'}
    if (isCompleted) {return 'Scanned'}
    return 'Not scanned'
  }

  return (
    <>
      <div className="border-b border-neutral-200 dark:border-neutral-700 last:border-b-0">
        {/* Header - clickable to expand */}
        <div
          className={cn(
            'p-4 cursor-pointer select-none transition-colors',
            'hover:bg-neutral-50 dark:hover:bg-neutral-800/50'
          )}
          onClick={() => setIsExpanded(!isExpanded)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === 'Enter' && setIsExpanded(!isExpanded)}
          aria-expanded={isExpanded}
        >
          <div className="flex items-start justify-between gap-4">
            {/* Left: Library info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h3 className="font-medium text-neutral-900 dark:text-neutral-100 truncate">
                  {library.name}
                </h3>
                <span className="text-xs px-2 py-0.5 rounded bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400">
                  {library.type}
                </span>
                {library.monitoring_enabled && library.last_scanned_at && (
                  <span
                    className="flex items-center gap-1.5 text-xs px-2 py-0.5 rounded bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400"
                    title="Filesystem monitoring active - changes are detected automatically"
                  >
                    <span className="inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
                    Monitoring
                  </span>
                )}
                {totalIssueCount > 0 && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      openIssuesDialog()
                    }}
                    className="cursor-pointer text-xs px-2 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 hover:bg-amber-200 dark:hover:bg-amber-900/50 transition-colors"
                  >
                    {totalIssueCount} {totalIssueCount === 1 ? 'issue' : 'issues'}
                  </button>
                )}
              </div>
              <p className="text-sm text-neutral-500 dark:text-neutral-500 truncate mt-0.5">
                {library.path}
              </p>
            </div>

            {/* Right: Status + Actions */}
            <div className="flex items-center gap-4 shrink-0">
              <span className={cn(
                'text-sm flex items-center gap-1.5',
                isCompleted && enrichmentComplete
                  ? 'text-green-600 dark:text-green-500'
                  : 'text-neutral-500 dark:text-neutral-400'
              )}>
                {isCompleted && enrichmentComplete && (
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                )}
                {getStatusSummary()}
              </span>

              {/* Action buttons */}
              <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
                {isPaused ? (
                  <Button size="sm" variant="primary" onClick={handleResume} isLoading={resumeMutation.isPending}>
                    Resume
                  </Button>
                ) : isScanning ? (
                  <Button size="sm" variant="secondary" onClick={handlePause} isLoading={pauseMutation.isPending}>
                    Pause
                  </Button>
                ) : (
                  <Button size="sm" variant="secondary" onClick={handleScan} isLoading={scanMutation.isPending}>
                    {isCompleted ? 'Rescan' : 'Scan'}
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setShowSettingsModal(true)}
                  title="Library settings"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={handleDelete}
                  isLoading={deleteMutation.isPending}
                  disabled={isScanning}
                >
                  Delete
                </Button>
              </div>

              {/* Expand indicator */}
              <svg
                className={cn(
                  'w-5 h-5 text-neutral-400 transition-transform',
                  isExpanded && 'rotate-180'
                )}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 9l-7 7-7-7" />
              </svg>
            </div>
          </div>

          {/* Inline progress bar when scanning/enriching (collapsed view) */}
          {!isExpanded && (isScanning || isPaused || isEnriching) && (
            <div className="mt-3" onClick={(e) => e.stopPropagation()}>
              <Progress
                value={
                  isScanning || isPaused
                    ? scanStatus?.progress ?? 0
                    : enrichmentProgress?.overallProgress?.percentage ?? 0
                }
                size="sm"
                variant={isPaused ? 'warning' : 'default'}
              />
              <p className="mt-1.5 text-xs text-neutral-500 dark:text-neutral-400">
                {(isScanning || isPaused) && scanStatus && (
                  <>
                    {scanStatus.filesProcessed.toLocaleString()}/{scanStatus.filesFound.toLocaleString()} files
                    {scanEtaDisplay && <span className="text-neutral-400 dark:text-neutral-500"> · {scanEtaDisplay}</span>}
                    {isEnriching && enrichmentProgress?.currentItem && (
                      <span className="text-neutral-400 dark:text-neutral-500">
                        {' · '}{formatStageName(enrichmentProgress.currentItem.stage)}: {enrichmentProgress.currentItem.title}
                      </span>
                    )}
                  </>
                )}
                {isEnriching && !isScanning && !isPaused && (
                  <>
                    {enrichmentProgress?.currentItem && (
                      <span>{formatStageName(enrichmentProgress.currentItem.stage)}: {enrichmentProgress.currentItem.title}</span>
                    )}
                    {enrichmentEtaDisplay && (
                      <span className="text-neutral-400 dark:text-neutral-500"> · {enrichmentEtaDisplay}</span>
                    )}
                  </>
                )}
              </p>
            </div>
          )}
        </div>

        {/* Expanded details */}
        {isExpanded && (
          <div className="px-4 pb-4 border-t border-neutral-100 dark:border-neutral-800">
            {/* Stats row */}
            <div className="flex gap-8 py-3 text-sm">
              {isCompleted && scanStatus && (
                <div className="flex flex-col">
                  <span className="text-xs text-neutral-500 dark:text-neutral-500 uppercase tracking-wide">Files</span>
                  <span className="font-medium text-neutral-900 dark:text-neutral-100">
                    {scanStatus.filesProcessed.toLocaleString()}
                  </span>
                </div>
              )}
              {enrichmentProgress?.overallProgress && (
                <div className="flex flex-col">
                  <span className="text-xs text-neutral-500 dark:text-neutral-500 uppercase tracking-wide">Enriched</span>
                  <span className="font-medium text-neutral-900 dark:text-neutral-100">
                    {enrichmentProgress.overallProgress.completedItems.toLocaleString()}/{enrichmentProgress.overallProgress.totalItems.toLocaleString()}
                  </span>
                </div>
              )}
            </div>

            {/* Active progress */}
            {(isScanning || isPaused) && scanStatus && (
              <div className="py-3 border-t border-neutral-100 dark:border-neutral-800">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300">
                    {isPaused ? 'Scan Paused' : 'Scanning'}
                  </span>
                  <span className="text-sm text-neutral-500">
                    {Math.round(scanStatus.progress)}%
                  </span>
                </div>
                <Progress
                  value={scanStatus.progress}
                  size="sm"
                  variant={isPaused ? 'warning' : 'default'}
                />
                <p className="text-xs text-neutral-500 mt-1">
                  {scanStatus.filesProcessed.toLocaleString()} / {scanStatus.filesFound.toLocaleString()} files
                  {scanEtaDisplay && ` - ${scanEtaDisplay} remaining`}
                </p>
              </div>
            )}

            {/* Enrichment progress */}
            {isEnriching && enrichmentProgress?.overallProgress && (
              <div className="py-3 border-t border-neutral-100 dark:border-neutral-800">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300">
                    Enriching
                  </span>
                  <span className="text-sm text-neutral-500">
                    {Math.round(enrichmentProgress.overallProgress.percentage)}%
                  </span>
                </div>
                <Progress
                  value={enrichmentProgress.overallProgress.percentage}
                  size="sm"
                />
                <p className="text-xs text-neutral-500 mt-1">
                  {enrichmentProgress.currentItem && (
                    <span>{formatStageName(enrichmentProgress.currentItem.stage)}: {enrichmentProgress.currentItem.title}</span>
                  )}
                  {enrichmentEtaDisplay && (
                    <span className="text-neutral-400 dark:text-neutral-500">
                      {enrichmentProgress.currentItem && ' · '}
                      {enrichmentEtaDisplay} remaining
                    </span>
                  )}
                </p>
              </div>
            )}

            {/* Stage breakdown */}
            {stages.length > 0 && (
              <div className="py-3 border-t border-neutral-100 dark:border-neutral-800">
                <h4 className="text-xs font-medium text-neutral-500 dark:text-neutral-500 uppercase tracking-wide mb-2">
                  Enrichment Stages
                </h4>
                <div className="space-y-1.5">
                  {stages.sort((a, b) => a.name.localeCompare(b.name)).map((stage) => (
                    <div key={stage.name} className="flex items-center justify-between text-sm">
                      <div className="flex items-center gap-2">
                        {/* Status dot */}
                        <span className={cn(
                          'w-2 h-2 rounded-full',
                          stage.isComplete ? 'bg-green-500' :
                          stage.isActive ? 'bg-blue-500' :
                          'bg-neutral-300 dark:bg-neutral-600'
                        )} />
                        <span className="text-neutral-700 dark:text-neutral-300">
                          {formatStageName(stage.name)}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-neutral-500 dark:text-neutral-500 tabular-nums">
                          {stage.completed.toLocaleString()}/{stage.total.toLocaleString()}
                        </span>
                        {stage.failed > 0 && (
                          <button
                            onClick={(e) => {
                              e.stopPropagation()
                              openIssuesDialog('enrichment')
                            }}
                            className="text-xs text-red-500 hover:text-red-600 dark:hover:text-red-400 hover:underline cursor-pointer"
                          >
                            {stage.failed} failed
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

          </div>
        )}
      </div>

      {totalIssueCount > 0 && library.id && (
        <ScanErrorsDialog
          libraryId={library.id}
          jobId={scanStatus?.jobId}
          isOpen={showErrorsDialog}
          onClose={() => {
            setShowErrorsDialog(false)
            setDialogInitialTab(undefined)
          }}
          onRetrySuccess={() => {
            invalidateLibraries()
            toast.success('Retrying failed items...')
          }}
          initialTab={dialogInitialTab}
        />
      )}

      <LibrarySettingsModal
        library={library}
        isOpen={showSettingsModal}
        onClose={() => setShowSettingsModal(false)}
        onSave={() => invalidateLibraries()}
      />
    </>
  )
}

export type { LibraryCardProps } from './LibraryCard.types'
export { LibraryCard }
