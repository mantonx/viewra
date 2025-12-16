import { EnrichmentIndicator } from '@/components/library/EnrichmentIndicator'
import { ScanErrorsDialog } from '@/components/library/ScanErrorsDialog'
import { Button, Progress } from '@/components/ui'
import {
  useDeleteApiLibrariesId,
  useGetApiMedia,
  usePostApiLibrariesIdScan,
  usePostApiLibrariesIdScanJobIdPause,
  usePostApiLibrariesIdScanJobIdResume,
} from '@/lib/api'
import { useConfirm } from '@/lib/hooks/useConfirm'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useScanProgress } from '@/lib/hooks/useScanProgress'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { pluralize, formatETA } from '@/lib/utils/format'
import { useState } from 'react'
import type { LibraryCardProps } from './LibraryCard.types'

const LibraryCard = ({ library }: LibraryCardProps) => {
  const [showErrorsDialog, setShowErrorsDialog] = useState(false)
  const invalidateLibraries = useInvalidateLibraries()
  const deleteMutation = useDeleteApiLibrariesId()
  const scanMutation = usePostApiLibrariesIdScan()
  const pauseMutation = usePostApiLibrariesIdScanJobIdPause()
  const resumeMutation = usePostApiLibrariesIdScanJobIdResume()
  const toast = useToast()
  const { confirm } = useConfirm()

  // Safe library ID with fallback (hooks require a number, but ID is optional in generated type)
  const libraryId = library.id ?? 0

  // Get real-time scan status via SSE
  const { scanStatus, isScanning, isPaused } = useScanProgress(libraryId, {
    enabled: libraryId > 0,
  })

  // Get total media count for this library
  const { data: mediaCount } = useGetApiMedia(
    { library_id: library.id, limit: 1 },
    {
      query: {
        enabled: !!library.id,
      },
    }
  )

  const handleDelete = async () => {
    if (!library.id || !library.name) {
      return
    }

    const confirmed = await confirm({
      title: 'Delete Library',
      message: `Are you sure you want to delete "${library.name}"? This action cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })

    if (!confirmed) {
      return
    }

    try {
      await deleteMutation.mutateAsync({ id: library.id })
      invalidateLibraries()
      toast.success('Library deleted successfully')
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to delete library'))
    }
  }

  const handleScan = async () => {
    if (!library.id) {
      return
    }
    try {
      await scanMutation.mutateAsync({ id: library.id })
      toast.success('Scan started successfully! The library will be updated shortly.')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to start scan'))
    }
  }

  const handlePause = async () => {
    if (!library.id || !scanStatus?.jobId) {
      return
    }
    try {
      await pauseMutation.mutateAsync({ id: library.id, jobId: scanStatus.jobId })
      toast.success('Scan paused')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to pause scan'))
    }
  }

  const handleResume = async () => {
    if (!library.id || !scanStatus?.jobId) {
      return
    }
    try {
      await resumeMutation.mutateAsync({ id: library.id, jobId: scanStatus.jobId })
      toast.success('Scan resumed')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to resume scan'))
    }
  }

  // Derive display states from SSE scan status (using camelCase from hook)
  const hasErrors = scanStatus && scanStatus.errorCount > 0
  const hasWarnings = scanStatus && scanStatus.warningCount > 0
  const hasIssues = hasErrors || hasWarnings
  const isCompleted = scanStatus?.isCompleted ?? false
  const totalMediaCount =
    mediaCount?.data && 'total' in mediaCount.data ? (mediaCount.data.total ?? 0) : 0

  // Discovery health checks
  const hasDiscoveryIssues =
    scanStatus &&
    (scanStatus.dirsSkipped > 0 ||
      scanStatus.discoveryErrors > 0 ||
      scanStatus.filesSkipped > 0)
  const discoveryWarningsCount =
    (scanStatus?.discoveryWarnings ?? 0) + (scanStatus?.discoveryErrors ?? 0)

  // Calculate ETA display
  const etaDisplay = scanStatus?.etaSeconds ? formatETA(scanStatus.etaSeconds) : null

  return (
    <>
      <div className="hover:bg-neutral-50 dark:hover:bg-neutral-800">
        <div className="p-4">
          <div className="flex justify-between items-start">
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold text-lg text-neutral-900 dark:text-neutral-50">{library.name}</h3>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">{library.path}</p>
              <div className="mt-2 flex gap-4 items-center text-sm text-neutral-500 dark:text-neutral-500 flex-wrap">
                <span>Type: {library.type}</span>
                {isCompleted && scanStatus && (
                  <span
                    className={
                      hasIssues ? 'text-yellow-600 dark:text-yellow-500 font-medium' : 'text-green-600 dark:text-green-500 font-medium'
                    }
                    title={`${scanStatus.filesProcessed.toLocaleString()} files processed, ${totalMediaCount.toLocaleString()} total media items in library`}
                  >
                    ✓ Scan complete ({scanStatus.filesProcessed.toLocaleString()}{' '}
                    {scanStatus.filesProcessed === 1 ? 'file' : 'files'})
                  </span>
                )}
                {hasErrors && scanStatus && (
                  <button
                    onClick={() => setShowErrorsDialog(true)}
                    className="cursor-pointer text-red-600 dark:text-red-500 font-medium hover:underline focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-1 rounded-sm flex items-center gap-1.5 transition-colors"
                    title={isScanning ? 'View errors (scan in progress)' : 'View scan errors'}
                    aria-label={`View ${pluralize(scanStatus.errorCount, 'error')}${isScanning ? ' from ongoing scan' : ''}`}
                  >
                    <svg
                      className="w-4 h-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                    <span>{pluralize(scanStatus.errorCount, 'error')}</span>
                  </button>
                )}
                {hasWarnings && scanStatus && (
                  <button
                    onClick={() => setShowErrorsDialog(true)}
                    className="cursor-pointer text-yellow-600 dark:text-yellow-500 font-medium hover:underline focus:outline-none focus:ring-2 focus:ring-yellow-500 focus:ring-offset-1 rounded-sm flex items-center gap-1.5 transition-colors"
                    title={isScanning ? 'View warnings (scan in progress)' : 'View scan warnings'}
                    aria-label={`View ${pluralize(scanStatus.warningCount, 'warning')}${isScanning ? ' from ongoing scan' : ''}`}
                  >
                    <svg
                      className="w-4 h-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    <span>{pluralize(scanStatus.warningCount, 'warning')}</span>
                  </button>
                )}
              </div>
            </div>
            <div className="flex gap-2 ml-4">
              {isPaused ? (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleResume}
                  isLoading={resumeMutation.isPending}
                >
                  Resume
                </Button>
              ) : isScanning ? (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={handlePause}
                  isLoading={pauseMutation.isPending}
                >
                  Pause
                </Button>
              ) : (
                <Button
                  variant="success"
                  size="sm"
                  onClick={handleScan}
                  isLoading={scanMutation.isPending}
                >
                  {isCompleted ? 'Rescan' : 'Scan'}
                </Button>
              )}
              <Button
                variant="danger"
                size="sm"
                onClick={handleDelete}
                isLoading={deleteMutation.isPending}
                disabled={isScanning}
              >
                Delete
              </Button>
            </div>
          </div>
        </div>
        {(isScanning || isPaused) && scanStatus && (
          <div className="px-4 pb-4 space-y-2">
            <Progress
              value={
                scanStatus.phase === 'discovering' && !scanStatus.discoveryDone ? 0 : scanStatus.progress
              }
              label={
                isPaused
                  ? 'Scan paused'
                  : scanStatus.phase === 'discovering' && !scanStatus.discoveryDone
                    ? `Discovering files... ${scanStatus.filesFound.toLocaleString()} found${
                        scanStatus.estimatedTotal > 0
                          ? ` (est. ${scanStatus.estimatedTotal.toLocaleString()} total)`
                          : ''
                      }`
                    : `${scanStatus.filesProcessed.toLocaleString()} / ${scanStatus.filesFound.toLocaleString()} files scanned${
                        etaDisplay ? ` • ETA ${etaDisplay}` : ''
                      }`
              }
              variant={isPaused ? 'warning' : 'default'}
              size="sm"
              showPercentage={scanStatus.phase !== 'discovering' || scanStatus.discoveryDone}
            />
            {/* Compact enrichment indicator during scan */}
            {libraryId > 0 && <EnrichmentIndicator libraryId={libraryId} compact />}
          </div>
        )}
        {/* Full enrichment progress when scan is not active */}
        {!isScanning && !isPaused && libraryId > 0 && (
          <EnrichmentIndicator libraryId={libraryId} />
        )}
        {isCompleted && hasDiscoveryIssues && scanStatus && (
          <div className="px-4 pb-4">
            <div className="bg-yellow-50 dark:bg-yellow-950 border border-yellow-200 dark:border-yellow-800 rounded-md p-3">
              <div className="flex items-start gap-2">
                <svg
                  className="w-5 h-5 text-yellow-600 dark:text-yellow-500 shrink-0 mt-0.5"
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
                <div className="flex-1">
                  <h4 className="text-sm font-medium text-yellow-800 dark:text-yellow-200">Incomplete File Discovery</h4>
                  <div className="mt-2 flex items-center gap-4 text-sm">
                    <div className="flex items-baseline gap-2">
                      <span className="text-green-700 dark:text-green-400 font-semibold text-lg">
                        {scanStatus.filesFound.toLocaleString()}
                      </span>
                      <span className="text-yellow-700 dark:text-yellow-300">files found</span>
                    </div>
                    <span className="text-yellow-400 dark:text-yellow-600">•</span>
                    <div className="flex items-baseline gap-2">
                      <span className="text-red-600 dark:text-red-400 font-semibold text-lg">
                        {scanStatus.filesSkipped.toLocaleString()}
                      </span>
                      <span className="text-yellow-700 dark:text-yellow-300">skipped</span>
                    </div>
                  </div>
                  <p className="text-sm text-yellow-700 dark:text-yellow-300 mt-2">
                    {scanStatus.dirsSkipped > 0 && (
                      <span>
                        Could not read {scanStatus.dirsSkipped.toLocaleString()}{' '}
                        {pluralize(scanStatus.dirsSkipped, 'directory', 'directories')}.{' '}
                      </span>
                    )}
                    Check permissions and network connectivity.
                  </p>
                  {discoveryWarningsCount > 0 && (
                    <p className="text-xs text-yellow-600 dark:text-yellow-400 mt-1">
                      {discoveryWarningsCount} {pluralize(discoveryWarningsCount, 'warning')}{' '}
                      detected during discovery
                    </p>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {hasIssues && library.id && (
        <ScanErrorsDialog
          libraryId={library.id}
          jobId={scanStatus?.jobId}
          isOpen={showErrorsDialog}
          onClose={() => setShowErrorsDialog(false)}
          onRetrySuccess={() => {
            invalidateLibraries()
            toast.success('Retrying failed files...')
          }}
        />
      )}
    </>
  )
}

export type { LibraryCardProps } from './LibraryCard.types'
export { LibraryCard }
