import { useState, useEffect } from 'react'
import {
  useGetApiLibrariesIdIssues,
  usePostApiLibrariesIdScanJobIdRetryFailed,
  type InternalApiHandlersScanErrorDetail,
  type InternalApiHandlersRetryFailedResponse,
} from '@/lib/api'
import { Modal, ModalContent, ModalFooter, Button, Loading, Alert } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { useEnrichmentFailures } from '@/lib/hooks/useEnrichmentFailures'
import { getErrorMessage } from '@/lib/utils/error'
import { formatFileSize, formatDate, formatRelativeTime, pluralize } from '@/lib/utils/format'
import { isScanErrorsResponse } from '@/lib/utils/type-guards'
import { ERROR_CATEGORY_COLORS } from '@/lib/constants/scan'
import { bg, text, border } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ScanErrorsDialogProps } from './ScanErrorsDialog.types'

type TabType = 'scan' | 'enrichment'

const ScanErrorsDialog = ({ libraryId, jobId, isOpen, onClose, onRetrySuccess }: ScanErrorsDialogProps) => {
  const [expandedCategory, setExpandedCategory] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<TabType>('scan')
  const toast = useToast()

  // Always use library-level issues endpoint (persistent across scans)
  // This shows all unresolved warnings/errors regardless of which scan they came from
  const { data: errors, isLoading, error } = useGetApiLibrariesIdIssues(
    libraryId,
    {
      query: {
        enabled: isOpen && !!libraryId,
        staleTime: 0,
        refetchOnMount: 'always',
      },
    }
  )

  // Fetch enrichment failures
  const {
    failures: enrichmentFailures,
    total: enrichmentFailureCount,
    isLoading: isLoadingEnrichment,
    error: enrichmentError,
    retryAll: retryEnrichmentAll,
    isRetrying: isRetryingEnrichment,
  } = useEnrichmentFailures({
    libraryId,
    enabled: isOpen && !!libraryId,
    limit: 100,
  })

  const retryMutation = usePostApiLibrariesIdScanJobIdRetryFailed()

  // Auto-switch to enrichment tab if no scan issues but has enrichment failures
  const errorData = errors?.data && isScanErrorsResponse(errors.data) ? errors.data : null
  const scanIssueCount = errorData?.total_errors ?? 0
  useEffect(() => {
    if (isOpen && scanIssueCount === 0 && enrichmentFailureCount > 0) {
      setActiveTab('enrichment')
    } else if (isOpen && scanIssueCount > 0) {
      setActiveTab('scan')
    }
  }, [isOpen, scanIssueCount, enrichmentFailureCount])

  const handleRetry = async () => {
    if (jobId === undefined) {
      toast.error('No active scan job to retry')
      return
    }
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

  const handleRetryEnrichment = () => {
    retryEnrichmentAll()
    toast.success('Retrying enrichment failures...')
  }

  const toggleCategory = (category: string) => {
    setExpandedCategory(expandedCategory === category ? null : category)
  }

  const getCategoryColor = (category: string) => {
    return ERROR_CATEGORY_COLORS[category] || ERROR_CATEGORY_COLORS.unknown
  }

  // Count warnings vs errors
  const warningCount = errorData ? Object.values(errorData.by_category || {}).flat().filter((item: InternalApiHandlersScanErrorDetail) => item.status === 'warning').length : 0
  const errorCount = (errorData?.total_errors || 0) - warningCount

  // Determine if we should show tabs
  const showTabs = scanIssueCount > 0 || enrichmentFailureCount > 0

  // Build dynamic title
  const getTitle = () => {
    if (!showTabs) {
      return 'Library Issues'
    }
    if (activeTab === 'scan') {
      return `Scan Issues (${scanIssueCount})`
    }
    return `Enrichment Failures (${enrichmentFailureCount})`
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={getTitle()}
      size="lg"
    >
      <ModalContent>
        {/* Tabs */}
        {showTabs && (
          <div className={cn('flex border-b mb-4', border.secondary)}>
            <button
              onClick={() => setActiveTab('scan')}
              className={cn(
                'cursor-pointer px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                activeTab === 'scan'
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : cn('border-transparent', text.tertiary, 'hover:text-neutral-700 dark:hover:text-neutral-300')
              )}
            >
              Scan Issues {scanIssueCount > 0 && <span className="ml-1 text-xs">({scanIssueCount})</span>}
            </button>
            <button
              onClick={() => setActiveTab('enrichment')}
              className={cn(
                'cursor-pointer px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                activeTab === 'enrichment'
                  ? 'border-orange-500 text-orange-600 dark:text-orange-400'
                  : cn('border-transparent', text.tertiary, 'hover:text-neutral-700 dark:hover:text-neutral-300')
              )}
            >
              Enrichment Failures {enrichmentFailureCount > 0 && <span className="ml-1 text-xs">({enrichmentFailureCount})</span>}
            </button>
          </div>
        )}

        {/* Scan Issues Tab */}
        {activeTab === 'scan' && (
          <>
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
              <div className={cn('text-center py-8', text.tertiary)}>
                No scan issues found
              </div>
            )}

            {errorData && (errorData.total_errors ?? 0) > 0 && errorData.by_category && (
              <div className="space-y-4">
                {Object.entries(errorData.by_category).map(([category, items]) => {
                  const typedItems = items as InternalApiHandlersScanErrorDetail[]
                  return (
                  <div key={category} className="border rounded-lg overflow-hidden shadow-sm">
                    <button
                      className={`cursor-pointer w-full px-4 py-3 flex items-center justify-between ${getCategoryColor(category)} border-b transition-colors hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500`}
                      onClick={() => toggleCategory(category)}
                      aria-expanded={expandedCategory === category}
                      aria-label={`${expandedCategory === category ? 'Collapse' : 'Expand'} ${category} category with ${typedItems.length} files`}
                    >
                      <div className="flex items-center gap-3">
                        <svg
                          className={`w-4 h-4 transition-transform ${expandedCategory === category ? 'rotate-90' : ''}`}
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                          aria-hidden="true"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                        </svg>
                        <span className="font-semibold capitalize">{category}</span>
                      </div>
                      <span className="text-sm font-medium">{pluralize(typedItems.length, 'file')}</span>
                    </button>

                    {expandedCategory === category && (
                      <div className={cn(bg.elevated)}>
                        <ul className={cn('divide-y', border.secondary)}>
                          {typedItems.map((item, idx) => {
                            const isWarning = item.status === 'warning'
                            const fileName = item.file_path?.split('/').pop() || item.file_path || 'Unknown file'
                            const filePath = item.file_path || ''
                            return (
                            <li key={idx} className={cn('p-4 transition-colors', bg.hover.subtle)}>
                              <div className="space-y-3">
                                <div className="flex items-start gap-3">
                                  {isWarning ? (
                                    <svg className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                    </svg>
                                  ) : (
                                    <svg className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                  )}
                                  <div className="flex-1 min-w-0 space-y-1">
                                    <p className={cn('text-sm font-medium break-words', text.primary)} title={filePath}>
                                      {fileName}
                                    </p>
                                    <p className={cn('text-xs font-mono break-all', text.tertiary)}>
                                      {filePath}
                                    </p>
                                    <div className={cn('flex flex-wrap gap-x-4 gap-y-1 text-xs', text.tertiary)}>
                                      <span>Size: {formatFileSize(item.file_size ?? 0)}</span>
                                      {item.processed_at && (
                                        <span>
                                          {isWarning ? 'Processed' : 'Failed'}: {formatDate(item.processed_at)}
                                        </span>
                                      )}
                                    </div>
                                  </div>
                                </div>
                                {item.error_message && (
                                  <div className={`${isWarning ? 'bg-yellow-50 dark:bg-yellow-950 border-yellow-200 dark:border-yellow-800' : 'bg-red-50 dark:bg-red-950 border-red-200 dark:border-red-800'} border rounded-md p-3 ml-8`}>
                                    <p className={`text-sm leading-relaxed ${isWarning ? 'text-yellow-900 dark:text-yellow-100' : 'text-red-900 dark:text-red-100'}`}>
                                      {item.error_message}
                                    </p>
                                  </div>
                                )}
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
          </>
        )}

        {/* Enrichment Failures Tab */}
        {activeTab === 'enrichment' && (
          <>
            {isLoadingEnrichment && (
              <div className="flex justify-center py-8">
                <Loading />
              </div>
            )}

            {enrichmentError && (
              <Alert variant="error">
                {getErrorMessage(enrichmentError, 'Failed to load enrichment failures')}
              </Alert>
            )}

            {!isLoadingEnrichment && enrichmentFailures.length === 0 && (
              <div className={cn('text-center py-8', text.tertiary)}>
                No enrichment failures found
              </div>
            )}

            {enrichmentFailures.length > 0 && (
              <div className="space-y-3">
                {enrichmentFailures.map((failure) => (
                  <div
                    key={failure.id}
                    className={cn('border rounded-lg p-4', border.secondary, bg.elevated)}
                  >
                    <div className="flex items-start gap-3">
                      <svg
                        className="w-5 h-5 text-orange-500 flex-shrink-0 mt-0.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
                        />
                      </svg>
                      <div className="flex-1 min-w-0 space-y-2">
                        <div className="flex items-start justify-between gap-2">
                          <p className={cn('text-sm font-medium', text.primary)}>
                            {failure.title}
                          </p>
                          <span className={cn('text-xs shrink-0', text.tertiary)}>
                            {formatRelativeTime(failure.lastAttemptAt)}
                          </span>
                        </div>
                        <div className={cn('flex flex-wrap gap-x-3 gap-y-1 text-xs', text.tertiary)}>
                          <span className="capitalize">{failure.mediaType}</span>
                          <span>Stage: {failure.stage}</span>
                          <span>Attempts: {failure.attempts}/{failure.maxAttempts}</span>
                          {failure.errorCategory && (
                            <span className="text-orange-600 dark:text-orange-400">
                              {failure.errorCategory}
                            </span>
                          )}
                        </div>
                        {failure.errorMessage && (
                          <div className="bg-orange-50 dark:bg-orange-950 border border-orange-200 dark:border-orange-800 rounded-md p-3">
                            <p className="text-sm leading-relaxed text-orange-900 dark:text-orange-100">
                              {failure.errorMessage}
                            </p>
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </ModalContent>

      <ModalFooter className="justify-between">
        {activeTab === 'scan' && errorData && (errorData.total_errors ?? 0) > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {errorCount > 0 && warningCount > 0 && (
                <>{pluralize(errorCount, 'error')} and {pluralize(warningCount, 'warning')}</>
              )}
              {errorCount > 0 && warningCount === 0 && (
                <>{pluralize(errorCount, 'file')} failed during scanning</>
              )}
              {errorCount === 0 && warningCount > 0 && (
                <>{pluralize(warningCount, 'file')} processed with warnings</>
              )}
            </p>
            <div className="flex gap-2">
              <Button onClick={onClose} variant="secondary">
                Close
              </Button>
              {errorCount > 0 && jobId !== undefined && (
                <Button
                  onClick={handleRetry}
                  variant="primary"
                  isLoading={retryMutation.isPending}
                >
                  Retry Failed Files
                </Button>
              )}
            </div>
          </>
        ) : activeTab === 'enrichment' && enrichmentFailureCount > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {pluralize(enrichmentFailureCount, 'enrichment failure')}
            </p>
            <div className="flex gap-2">
              <Button onClick={onClose} variant="secondary">
                Close
              </Button>
              <Button
                onClick={handleRetryEnrichment}
                variant="primary"
                isLoading={isRetryingEnrichment}
              >
                Retry All
              </Button>
            </div>
          </>
        ) : (
          <Button onClick={onClose} variant="secondary" className="ml-auto">
            Close
          </Button>
        )}
      </ModalFooter>
    </Modal>
  )
}

export { ScanErrorsDialog }
export type { ScanErrorsDialogProps } from './ScanErrorsDialog.types'
