import { useState } from 'react'
import {
  useGetApiLibrariesIdScanJobIdErrors,
  usePostApiLibrariesIdScanJobIdRetryFailed,
  type InternalApiHandlersScanErrorDetail,
  type InternalApiHandlersScanErrorsResponse,
  type InternalApiHandlersRetryFailedResponse,
} from '@/lib/api'
import { Modal, Button, Loading, Alert } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import type { ScanErrorsDialogProps } from './ScanErrorsDialog.types'

const ScanErrorsDialog = ({ libraryId, jobId, isOpen, onClose, onRetrySuccess }: ScanErrorsDialogProps) => {
  const [expandedCategory, setExpandedCategory] = useState<string | null>(null)
  const toast = useToast()

  const { data: errors, isLoading, error } = useGetApiLibrariesIdScanJobIdErrors(
    libraryId,
    jobId,
    {
      query: {
        enabled: isOpen && !!libraryId && !!jobId,
      },
    }
  )

  const retryMutation = usePostApiLibrariesIdScanJobIdRetryFailed()

  const handleRetry = async () => {
    try {
      const response = await retryMutation.mutateAsync({
        id: libraryId,
        jobId,
      })
      // Type guard for retry response
      const retryData = response.data as InternalApiHandlersRetryFailedResponse
      toast.success(retryData.message || `${retryData.count} files queued for retry`)
      onRetrySuccess?.()
      onClose()
    } catch (err) {
      toast.error(getErrorMessage(err, 'Failed to retry failed files'))
    }
  }

  // Type guard to check if response is successful
  const isScanErrorsResponse = (data: any): data is InternalApiHandlersScanErrorsResponse => {
    return data && typeof data.total_errors === 'number' && data.by_category
  }

  const errorData = errors?.data && isScanErrorsResponse(errors.data) ? errors.data : null

  const toggleCategory = (category: string) => {
    setExpandedCategory(expandedCategory === category ? null : category)
  }

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
  }

  const getCategoryColor = (category: string) => {
    const colors: Record<string, string> = {
      ffmpeg: 'text-red-600 bg-red-50 border-red-200',
      parsing: 'text-orange-600 bg-orange-50 border-orange-200',
      database: 'text-purple-600 bg-purple-50 border-purple-200',
      filesystem: 'text-blue-600 bg-blue-50 border-blue-200',
      metadata: 'text-yellow-600 bg-yellow-50 border-yellow-200',
      unknown: 'text-gray-600 bg-gray-50 border-gray-200',
    }
    return colors[category] || colors.unknown
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Scan Errors (${errorData?.total_errors || 0})`}
      size="lg"
    >
      <div className="max-h-[600px] overflow-y-auto">
        {isLoading && (
          <div className="flex justify-center py-8">
            <Loading />
          </div>
        )}

        {error && (
          <Alert variant="error">
            {getErrorMessage(error, 'Failed to load scan errors')}
          </Alert>
        )}

        {errorData && (errorData.total_errors ?? 0) === 0 && (
          <div className="text-center py-8 text-gray-500">
            No errors found for this scan
          </div>
        )}

        {errorData && (errorData.total_errors ?? 0) > 0 && errorData.by_category && (
          <div className="space-y-3">
            {Object.entries(errorData.by_category).map(([category, items]) => {
              const typedItems = items as InternalApiHandlersScanErrorDetail[]
              return (
              <div key={category} className="border rounded-lg overflow-hidden">
                <button
                  className={`w-full p-4 flex items-center justify-between ${getCategoryColor(category)} border-b transition-colors hover:opacity-80`}
                  onClick={() => toggleCategory(category)}
                >
                  <div className="flex items-center gap-2">
                    <svg
                      className={`w-5 h-5 transition-transform ${expandedCategory === category ? 'rotate-90' : ''}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>
                    <span className="font-semibold capitalize">{category}</span>
                  </div>
                  <span className="text-sm font-medium">{typedItems.length} files</span>
                </button>

                {expandedCategory === category && (
                  <div className="bg-white">
                    <ul className="divide-y divide-gray-200">
                      {typedItems.map((item, idx) => (
                        <li key={idx} className="p-4 hover:bg-gray-50">
                          <div className="space-y-2">
                            <div className="flex items-start justify-between gap-4">
                              <div className="flex-1 min-w-0">
                                <p className="text-sm font-mono text-gray-900 truncate" title={item.file_path ?? ''}>
                                  {item.file_path}
                                </p>
                                <p className="text-xs text-gray-500 mt-1">
                                  Size: {formatFileSize(item.file_size ?? 0)}
                                  {item.processed_at && (
                                    <span className="ml-3">
                                      Failed: {new Date(item.processed_at).toLocaleString()}
                                    </span>
                                  )}
                                </p>
                              </div>
                            </div>
                            <div className="bg-red-50 border border-red-200 rounded p-2">
                              <p className="text-sm text-red-800">{item.error_message}</p>
                            </div>
                          </div>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )})}
          </div>
        )}
      </div>

      {errorData && (errorData.total_errors ?? 0) > 0 && (
        <div className="mt-6 flex justify-between items-center border-t pt-4">
          <p className="text-sm text-gray-600">
            {errorData.total_errors ?? 0} file{(errorData.total_errors ?? 0) !== 1 ? 's' : ''} failed during scanning
          </p>
          <div className="flex gap-2">
            <Button onClick={onClose} variant="secondary">
              Close
            </Button>
            <Button
              onClick={handleRetry}
              variant="primary"
              isLoading={retryMutation.isPending}
            >
              Retry Failed Files
            </Button>
          </div>
        </div>
      )}

      {(!errorData || (errorData.total_errors ?? 0) === 0) && (
        <div className="mt-4 flex justify-end">
          <Button onClick={onClose} variant="secondary">
            Close
          </Button>
        </div>
      )}
    </Modal>
  )
}

export { ScanErrorsDialog }
export type { ScanErrorsDialogProps } from './ScanErrorsDialog.types'
