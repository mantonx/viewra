import { useState } from 'react'
import {
  useGetApiLibrariesIdScanJobIdErrors,
  usePostApiLibrariesIdScanJobIdRetryFailed,
  type InternalApiHandlersScanErrorDetail,
  type InternalApiHandlersRetryFailedResponse,
} from '@/lib/api'
import { Modal, Button, Loading, Alert } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { formatFileSize, pluralize } from '@/lib/utils/format'
import { isScanErrorsResponse } from '@/lib/utils/type-guards'
import { ERROR_CATEGORY_COLORS } from '@/lib/constants/scan'
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
      const retryData = response.data as InternalApiHandlersRetryFailedResponse
      toast.success(retryData.message || `${retryData.count} files queued for retry`)
      onRetrySuccess?.()
      onClose()
    } catch (err) {
      toast.error(getErrorMessage(err, 'Failed to retry failed files'))
    }
  }

  const errorData = errors?.data && isScanErrorsResponse(errors.data) ? errors.data : null

  const toggleCategory = (category: string) => {
    setExpandedCategory(expandedCategory === category ? null : category)
  }

  const getCategoryColor = (category: string) => {
    return ERROR_CATEGORY_COLORS[category] || ERROR_CATEGORY_COLORS.unknown
  }

  // Count warnings vs errors
  const warningCount = errorData ? Object.values(errorData.by_category || {}).flat().filter((item: any) => item.status === 'warning').length : 0
  const errorCount = (errorData?.total_errors || 0) - warningCount

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Scan Issues (${errorData?.total_errors || 0})`}
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
                      {typedItems.map((item, idx) => {
                        const isWarning = item.status === 'warning'
                        return (
                        <li key={idx} className="p-4 hover:bg-gray-50">
                          <div className="space-y-2">
                            <div className="flex items-start justify-between gap-4">
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                  {isWarning ? (
                                    <svg className="w-4 h-4 text-yellow-600 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                    </svg>
                                  ) : (
                                    <svg className="w-4 h-4 text-red-600 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                  )}
                                  <p className="text-sm font-mono text-gray-900 truncate" title={item.file_path ?? ''}>
                                    {item.file_path}
                                  </p>
                                </div>
                                <p className="text-xs text-gray-500 mt-1 ml-6">
                                  Size: {formatFileSize(item.file_size ?? 0)}
                                  {item.processed_at && (
                                    <span className="ml-3">
                                      {isWarning ? 'Processed' : 'Failed'}: {new Date(item.processed_at).toLocaleString()}
                                    </span>
                                  )}
                                </p>
                              </div>
                            </div>
                            <div className={`${isWarning ? 'bg-yellow-50 border-yellow-200' : 'bg-red-50 border-red-200'} border rounded p-2 ml-6`}>
                              <p className={`text-sm ${isWarning ? 'text-yellow-800' : 'text-red-800'}`}>{item.error_message}</p>
                            </div>
                          </div>
                        </li>
                      )})}
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
            {pluralize(errorData.total_errors, 'file')} failed during scanning
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
