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

import { bg, text, border } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ScanErrorsDialogProps, IssueTab } from './ScanErrorsDialog.types'

const ScanErrorsDialog = ({ libraryId, jobId, isOpen, onClose, onRetrySuccess, initialTab }: ScanErrorsDialogProps) => {
  const [activeTab, setActiveTab] = useState<IssueTab>('errors')
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
    retrySingle: retryEnrichmentSingle,
    isRetryingSingle: isRetryingEnrichmentSingle,
  } = useEnrichmentFailures({
    libraryId,
    enabled: isOpen && !!libraryId,
    limit: 100,
  })

  const retryMutation = usePostApiLibrariesIdScanJobIdRetryFailed()

  // Parse scan issues into separate errors vs warnings
  const errorData = errors?.data && isScanErrorsResponse(errors.data) ? errors.data : null
  
  // Split items by status (error vs warning)
  const allItems = errorData?.by_category
    ? Object.entries(errorData.by_category).flatMap(([category, items]) =>
        (items as InternalApiHandlersScanErrorDetail[]).map((item) => ({ ...item, category }))
      )
    : []
  
  const scanErrors = allItems.filter((item) => item.status !== 'warning')
  const scanWarnings = allItems.filter((item) => item.status === 'warning')
  
  const scanErrorCount = scanErrors.length
  const scanWarningCount = scanWarnings.length

  // Set initial tab when dialog opens
  useEffect(() => {
    if (isOpen) {
      // If initialTab is specified, use it
      if (initialTab) {
        setActiveTab(initialTab)
      } else {
        // Otherwise auto-switch to first tab with issues
        if (scanErrorCount > 0) {
          setActiveTab('errors')
        } else if (scanWarningCount > 0) {
          setActiveTab('warnings')
        } else if (enrichmentFailureCount > 0) {
          setActiveTab('enrichment')
        } else {
          setActiveTab('errors')
        }
      }
    }
  }, [isOpen, initialTab, scanErrorCount, scanWarningCount, enrichmentFailureCount])

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

  // Determine if we should show tabs (at least one category has issues)
  const showTabs = scanErrorCount > 0 || scanWarningCount > 0 || enrichmentFailureCount > 0

  // Build dynamic title based on active tab
  const getTitle = () => {
    if (!showTabs) {
      return 'Library Issues'
    }
    switch (activeTab) {
      case 'errors':
        return `Scan Errors (${scanErrorCount})`
      case 'warnings':
        return `Scan Warnings (${scanWarningCount})`
      case 'enrichment':
        return `Enrichment Failures (${enrichmentFailureCount})`
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={getTitle()}
      size="lg"
    >
      <ModalContent>
        {/* Tabs - 3 tabs: Scan Errors, Scan Warnings, Enrichment Failures */}
        {showTabs && (
          <div className={cn('flex border-b mb-4', border.secondary)}>
            <button
              onClick={() => setActiveTab('errors')}
              className={cn(
                'cursor-pointer px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                activeTab === 'errors'
                  ? 'border-red-500 text-red-600 dark:text-red-400'
                  : cn('border-transparent', text.tertiary, 'hover:text-neutral-700 dark:hover:text-neutral-300')
              )}
            >
              Scan Errors {scanErrorCount > 0 && <span className="ml-1 text-xs">({scanErrorCount})</span>}
            </button>
            <button
              onClick={() => setActiveTab('warnings')}
              className={cn(
                'cursor-pointer px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                activeTab === 'warnings'
                  ? 'border-yellow-500 text-yellow-600 dark:text-yellow-400'
                  : cn('border-transparent', text.tertiary, 'hover:text-neutral-700 dark:hover:text-neutral-300')
              )}
            >
              Scan Warnings {scanWarningCount > 0 && <span className="ml-1 text-xs">({scanWarningCount})</span>}
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
              Enrichment {enrichmentFailureCount > 0 && <span className="ml-1 text-xs">({enrichmentFailureCount})</span>}
            </button>
          </div>
        )}

        {/* Scan Errors Tab */}
        {activeTab === 'errors' && (
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

            {!isLoading && scanErrorCount === 0 && (
              <div className={cn('text-center py-8', text.tertiary)}>
                No scan errors found
              </div>
            )}

            {scanErrorCount > 0 && (
              <div className="space-y-3">
                {scanErrors.map((item, idx) => {
                  const fileName = item.file_path?.split('/').pop() || item.file_path || 'Unknown file'
                  const filePath = item.file_path || ''
                  return (
                    <div key={idx} className={cn('border rounded-lg p-4', border.secondary, bg.elevated)}>
                      <div className="flex items-start gap-3">
                        <svg className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <div className="flex-1 min-w-0 space-y-2">
                          <p className={cn('text-sm font-medium break-words', text.primary)} title={filePath}>
                            {fileName}
                          </p>
                          <p className={cn('text-xs font-mono break-all', text.tertiary)}>
                            {filePath}
                          </p>
                          <div className={cn('flex flex-wrap gap-x-4 gap-y-1 text-xs', text.tertiary)}>
                            <span className="capitalize">{item.category}</span>
                            <span>Size: {formatFileSize(item.file_size ?? 0)}</span>
                            {item.processed_at && (
                              <span>Failed: {formatDate(item.processed_at)}</span>
                            )}
                          </div>
                          {item.error_message && (
                            <div className="bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded-md p-3">
                              <p className="text-sm leading-relaxed text-red-900 dark:text-red-100">
                                {item.error_message}
                              </p>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </>
        )}

        {/* Scan Warnings Tab */}
        {activeTab === 'warnings' && (
          <>
            {isLoading && (
              <div className="flex justify-center py-8">
                <Loading />
              </div>
            )}

            {error && (
              <Alert variant="error">
                {getErrorMessage(error, 'Failed to load scan warnings')}
              </Alert>
            )}

            {!isLoading && scanWarningCount === 0 && (
              <div className={cn('text-center py-8', text.tertiary)}>
                No scan warnings found
              </div>
            )}

            {scanWarningCount > 0 && (
              <div className="space-y-3">
                {scanWarnings.map((item, idx) => {
                  const fileName = item.file_path?.split('/').pop() || item.file_path || 'Unknown file'
                  const filePath = item.file_path || ''
                  return (
                    <div key={idx} className={cn('border rounded-lg p-4', border.secondary, bg.elevated)}>
                      <div className="flex items-start gap-3">
                        <svg className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                        </svg>
                        <div className="flex-1 min-w-0 space-y-2">
                          <p className={cn('text-sm font-medium break-words', text.primary)} title={filePath}>
                            {fileName}
                          </p>
                          <p className={cn('text-xs font-mono break-all', text.tertiary)}>
                            {filePath}
                          </p>
                          <div className={cn('flex flex-wrap gap-x-4 gap-y-1 text-xs', text.tertiary)}>
                            <span className="capitalize">{item.category}</span>
                            <span>Size: {formatFileSize(item.file_size ?? 0)}</span>
                            {item.processed_at && (
                              <span>Processed: {formatDate(item.processed_at)}</span>
                            )}
                          </div>
                          {item.error_message && (
                            <div className="bg-yellow-50 dark:bg-yellow-950 border border-yellow-200 dark:border-yellow-800 rounded-md p-3">
                              <p className="text-sm leading-relaxed text-yellow-900 dark:text-yellow-100">
                                {item.error_message}
                              </p>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })}
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
                {enrichmentFailures.map((failure) => {
                  const isRetrying = isRetryingEnrichmentSingle(failure.id)
                  return (
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
                            <div className="flex items-center gap-2 shrink-0">
                              <span className={cn('text-xs', text.tertiary)}>
                                {formatRelativeTime(failure.lastAttemptAt)}
                              </span>
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() => retryEnrichmentSingle(failure.id)}
                                isLoading={isRetrying}
                                disabled={isRetrying}
                                title="Retry this item"
                              >
                                Retry
                              </Button>
                            </div>
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
                  )
                })}
              </div>
            )}
          </>
        )}
      </ModalContent>

      <ModalFooter className="justify-between">
        {activeTab === 'errors' && scanErrorCount > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {pluralize(scanErrorCount, 'file')} failed during scanning
            </p>
            <div className="flex gap-2">
              <Button onClick={onClose} variant="secondary">
                Close
              </Button>
              {jobId !== undefined && (
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
        ) : activeTab === 'warnings' && scanWarningCount > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {pluralize(scanWarningCount, 'file')} processed with warnings
            </p>
            <div className="flex gap-2">
              <Button onClick={onClose} variant="secondary">
                Close
              </Button>
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
export type { ScanErrorsDialogProps, IssueTab } from './ScanErrorsDialog.types'
