import { ScanErrorsDialog } from '@/components/library/ScanErrorsDialog'
import { Button, ProgressBar } from '@/components/ui'
import {
  useDeleteApiLibrariesId,
  useGetApiLibrariesIdScanStatus,
  useGetApiMedia,
  usePostApiLibrariesIdScan,
  usePostApiLibrariesIdScanJobIdPause,
  usePostApiLibrariesIdScanJobIdResume,
} from '@/lib/api'
import { SCAN_POLL_INTERVAL_MS } from '@/lib/constants/scan'
import { useConfirm } from '@/lib/hooks/useConfirm'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { pluralize, formatETA } from '@/lib/utils/format'
import { isScanStatusResponse } from '@/lib/utils/type-guards'
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

  // Get latest scan status to show error count
  const { data: scanStatus } = useGetApiLibrariesIdScanStatus(library.id!, {
    query: {
      enabled: !!library.id,
      refetchInterval: SCAN_POLL_INTERVAL_MS,
    },
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
    if (!library.id || !scanData?.job_id) {
      return
    }
    try {
      await pauseMutation.mutateAsync({ id: library.id, jobId: scanData.job_id })
      toast.success('Scan paused')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to pause scan'))
    }
  }

  const handleResume = async () => {
    if (!library.id || !scanData?.job_id) {
      return
    }
    try {
      await resumeMutation.mutateAsync({ id: library.id, jobId: scanData.job_id })
      toast.success('Scan resumed')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to resume scan'))
    }
  }

  const scanData =
    scanStatus?.data && isScanStatusResponse(scanStatus.data) ? scanStatus.data : null
  const hasErrors = scanData && (scanData.error_count ?? 0) > 0
  const hasWarnings = scanData && (scanData.warning_count ?? 0) > 0
  const hasIssues = hasErrors || hasWarnings
  const isScanning = scanData?.status === 'running'
  const isPaused = scanData?.status === 'paused'
  const isCompleted = scanData?.status === 'completed'
  const totalMediaCount =
    mediaCount?.data && 'total' in mediaCount.data ? (mediaCount.data.total ?? 0) : 0

  // Discovery health checks
  const hasDiscoveryIssues =
    scanData &&
    ((scanData.dirs_skipped ?? 0) > 0 ||
      (scanData.discovery_errors ?? 0) > 0 ||
      (scanData.files_skipped ?? 0) > 0)
  const discoveryWarningsCount =
    (scanData?.discovery_warnings ?? 0) + (scanData?.discovery_errors ?? 0)

  // Calculate ETA display
  const etaDisplay = scanData?.eta_seconds ? formatETA(scanData.eta_seconds) : null

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
                {isCompleted && scanData && (
                  <span
                    className={
                      hasIssues ? 'text-yellow-600 dark:text-yellow-500 font-medium' : 'text-green-600 dark:text-green-500 font-medium'
                    }
                    title={`${(scanData.files_processed ?? 0).toLocaleString()} files processed, ${totalMediaCount.toLocaleString()} total media items in library`}
                  >
                    ✓ Scan complete ({(scanData.files_processed ?? 0).toLocaleString()}{' '}
                    {(scanData.files_processed ?? 0) === 1 ? 'file' : 'files'})
                  </span>
                )}
                {hasErrors && scanData && (
                  <button
                    onClick={() => setShowErrorsDialog(true)}
                    className="cursor-pointer text-red-600 dark:text-red-500 font-medium hover:underline focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-1 rounded-sm flex items-center gap-1.5 transition-colors"
                    title={isScanning ? 'View errors (scan in progress)' : 'View scan errors'}
                    aria-label={`View ${pluralize(scanData.error_count, 'error')}${isScanning ? ' from ongoing scan' : ''}`}
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
                    <span>{pluralize(scanData.error_count, 'error')}</span>
                  </button>
                )}
                {hasWarnings && scanData && (
                  <button
                    onClick={() => setShowErrorsDialog(true)}
                    className="cursor-pointer text-yellow-600 dark:text-yellow-500 font-medium hover:underline focus:outline-none focus:ring-2 focus:ring-yellow-500 focus:ring-offset-1 rounded-sm flex items-center gap-1.5 transition-colors"
                    title={isScanning ? 'View warnings (scan in progress)' : 'View scan warnings'}
                    aria-label={`View ${pluralize(scanData.warning_count, 'warning')}${isScanning ? ' from ongoing scan' : ''}`}
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
                    <span>{pluralize(scanData.warning_count, 'warning')}</span>
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
        {(isScanning || isPaused) && scanData && (
          <div className="px-4 pb-4">
            <ProgressBar
              progress={
                scanData.phase === 'discovering' && !scanData.discovery_done ? 0 : scanData.progress ?? 0
              }
              label={
                isPaused
                  ? 'Scan paused'
                  : scanData.phase === 'discovering' && !scanData.discovery_done
                    ? `Discovering files... ${(scanData.files_found ?? 0).toLocaleString()} found${
                        (scanData.estimated_total ?? 0) > 0
                          ? ` (est. ${(scanData.estimated_total ?? 0).toLocaleString()} total)`
                          : ''
                      }`
                    : `${(scanData.files_processed ?? 0).toLocaleString()} / ${(scanData.files_found ?? 0).toLocaleString()} files scanned${
                        etaDisplay ? ` • ETA ${etaDisplay}` : ''
                      }`
              }
              variant={isPaused ? 'warning' : 'default'}
              size="sm"
              showPercentage={scanData.phase !== 'discovering' || scanData.discovery_done}
            />
          </div>
        )}
        {isCompleted && hasDiscoveryIssues && scanData && (
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
                        {(scanData.files_found ?? 0).toLocaleString()}
                      </span>
                      <span className="text-yellow-700 dark:text-yellow-300">files found</span>
                    </div>
                    <span className="text-yellow-400 dark:text-yellow-600">•</span>
                    <div className="flex items-baseline gap-2">
                      <span className="text-red-600 dark:text-red-400 font-semibold text-lg">
                        {(scanData.files_skipped ?? 0).toLocaleString()}
                      </span>
                      <span className="text-yellow-700 dark:text-yellow-300">skipped</span>
                    </div>
                  </div>
                  <p className="text-sm text-yellow-700 dark:text-yellow-300 mt-2">
                    {scanData.dirs_skipped && scanData.dirs_skipped > 0 && (
                      <span>
                        Could not read {scanData.dirs_skipped.toLocaleString()}{' '}
                        {pluralize(scanData.dirs_skipped, 'directory', 'directories')}.{' '}
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
          jobId={scanData?.job_id}
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
