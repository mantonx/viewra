import { useState } from 'react'
import { useDeleteApiLibrariesId, usePostApiLibrariesIdScan, useGetApiLibrariesIdScanStatus, type InternalApiHandlersScanStatusResponse } from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useToast } from '@/lib/hooks/useToast'
import { useConfirm } from '@/lib/hooks/useConfirm'
import { getErrorMessage } from '@/lib/utils/error'
import { Button } from '@/components/ui'
import { ScanErrorsDialog } from '@/components/library/ScanErrorsDialog'
import type { LibraryCardProps } from './LibraryCard.types'

const LibraryCard = ({ library }: LibraryCardProps) => {
  const [showErrorsDialog, setShowErrorsDialog] = useState(false)
  const invalidateLibraries = useInvalidateLibraries()
  const deleteMutation = useDeleteApiLibrariesId()
  const scanMutation = usePostApiLibrariesIdScan()
  const toast = useToast()
  const { confirm } = useConfirm()

  // Get latest scan status to show error count
  const { data: scanStatus } = useGetApiLibrariesIdScanStatus(
    library.id!,
    {
      query: {
        enabled: !!library.id,
        refetchInterval: 5000, // Poll every 5 seconds if scan is running
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

  // Type guard to check if response is a successful scan status
  const isScanStatusResponse = (data: any): data is InternalApiHandlersScanStatusResponse => {
    return data && typeof data.error_count === 'number' && typeof data.status === 'string'
  }

  const scanData = scanStatus?.data && isScanStatusResponse(scanStatus.data) ? scanStatus.data : null
  const hasErrors = scanData && (scanData.error_count ?? 0) > 0
  const isScanning = scanData?.status === 'running'

  return (
    <>
      <div className="p-4 hover:bg-gray-50">
        <div className="flex justify-between items-start">
          <div className="flex-1">
            <h3 className="font-semibold text-lg">{library.name}</h3>
            <p className="text-sm text-gray-600">{library.path}</p>
            <div className="mt-2 flex gap-4 text-sm text-gray-500">
              <span>Type: {library.type}</span>
              {isScanning && scanData && (
                <span className="text-blue-600 font-medium">
                  Scanning... {(scanData.progress ?? 0).toFixed(1)}%
                </span>
              )}
              {hasErrors && !isScanning && scanData && (
                <button
                  onClick={() => setShowErrorsDialog(true)}
                  className="text-red-600 font-medium hover:underline flex items-center gap-1"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  {scanData.error_count ?? 0} error{(scanData.error_count ?? 0) !== 1 ? 's' : ''}
                </button>
              )}
            </div>
          </div>
          <div className="flex gap-2">
            <Button
              variant="success"
              size="sm"
              onClick={handleScan}
              isLoading={scanMutation.isPending || isScanning}
              disabled={isScanning}
            >
              {isScanning ? 'Scanning...' : 'Scan'}
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={handleDelete}
              isLoading={deleteMutation.isPending}
            >
              Delete
            </Button>
          </div>
        </div>
      </div>

      {hasErrors && library.id && scanData && scanData.job_id && (
        <ScanErrorsDialog
          libraryId={library.id}
          jobId={scanData.job_id}
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

export { LibraryCard }
export type { LibraryCardProps } from './LibraryCard.types'
