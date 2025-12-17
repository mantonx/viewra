import { useState } from 'react'
import {
  useGetApiLibrariesIdIssues,
  useGetApiLibrariesIdEnrichmentIssues,
  usePostApiLibrariesIdScanJobIdRetryFailed,
  type InternalApiHandlersScanErrorDetail,
  type InternalApiHandlersRetryFailedResponse,
  type InternalApiHandlersEnrichmentIssueDetail,
} from '@/lib/api'
import { Modal, ModalContent, ModalFooter, Button, Loading, Alert, Tabs, TabPanel } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { formatFileSize, formatDate, pluralize } from '@/lib/utils/format'
import { isScanErrorsResponse, isEnrichmentIssuesResponse } from '@/lib/utils/type-guards'
import { ERROR_CATEGORY_COLORS } from '@/lib/constants/scan'
import { bg, text, border } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { LibraryIssuesDialogProps } from './LibraryIssuesDialog.types'

/** Color mappings for enrichment stages */
const ENRICHMENT_STAGE_COLORS: Record<string, string> = {
  nfo: 'text-blue-600 bg-blue-50 border-blue-200 dark:bg-blue-950 dark:border-blue-800',
  localimages: 'text-green-600 bg-green-50 border-green-200 dark:bg-green-950 dark:border-green-800',
  tmdb: 'text-purple-600 bg-purple-50 border-purple-200 dark:bg-purple-950 dark:border-purple-800',
  unknown: 'text-gray-600 bg-gray-50 border-gray-200 dark:bg-gray-950 dark:border-gray-800',
}

const getStageColor = (stage: string) => {
  return ENRICHMENT_STAGE_COLORS[stage] || ENRICHMENT_STAGE_COLORS.unknown
}

const formatStageName = (stage: string) => {
  const names: Record<string, string> = {
    nfo: 'NFO Parsing',
    localimages: 'Local Images',
    tmdb: 'TMDB Lookup',
  }
  return names[stage] || stage.charAt(0).toUpperCase() + stage.slice(1)
}

const LibraryIssuesDialog = ({
  libraryId,
  jobId,
  isOpen,
  onClose,
  onRetrySuccess,
  initialTab = 'scan',
  scanErrorCount,
  enrichmentErrorCount,
}: LibraryIssuesDialogProps) => {
  const [activeTab, setActiveTab] = useState<string>(initialTab)
  const [expandedCategory, setExpandedCategory] = useState<string | null>(null)
  const [expandedStage, setExpandedStage] = useState<string | null>(null)
  const toast = useToast()

  // Fetch scan issues
  const {
    data: scanErrors,
    isLoading: scanLoading,
    error: scanError,
  } = useGetApiLibrariesIdIssues(libraryId, {
    query: {
      enabled: isOpen && !!libraryId,
      staleTime: 0,
      refetchOnMount: 'always',
    },
  })

  // Fetch enrichment issues
  const {
    data: enrichmentIssues,
    isLoading: enrichmentLoading,
    error: enrichmentError,
  } = useGetApiLibrariesIdEnrichmentIssues(libraryId, {
    query: {
      enabled: isOpen && !!libraryId,
      staleTime: 0,
      refetchOnMount: 'always',
    },
  })

  const retryMutation = usePostApiLibrariesIdScanJobIdRetryFailed()

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

  const scanData = scanErrors?.data && isScanErrorsResponse(scanErrors.data) ? scanErrors.data : null
  const enrichmentData =
    enrichmentIssues?.data && isEnrichmentIssuesResponse(enrichmentIssues.data)
      ? enrichmentIssues.data
      : null

  // Calculate counts from fetched data if not provided
  const actualScanCount = scanErrorCount ?? (scanData?.total_errors ?? 0)
  const actualEnrichmentCount = enrichmentErrorCount ?? (enrichmentData?.total_errors ?? 0)
  const totalIssues = actualScanCount + actualEnrichmentCount

  // Count warnings vs errors for scan
  const warningCount = scanData
    ? Object.values(scanData.by_category || {})
        .flat()
        .filter((item: InternalApiHandlersScanErrorDetail) => item.status === 'warning').length
    : 0
  const errorCount = (scanData?.total_errors || 0) - warningCount

  const toggleCategory = (category: string) => {
    setExpandedCategory(expandedCategory === category ? null : category)
  }

  const toggleStage = (stage: string) => {
    setExpandedStage(expandedStage === stage ? null : stage)
  }

  const getCategoryColor = (category: string) => {
    return ERROR_CATEGORY_COLORS[category] || ERROR_CATEGORY_COLORS.unknown
  }

  const tabs = [
    {
      id: 'scan',
      label: 'Scan Issues',
      badge: actualScanCount > 0 ? actualScanCount : undefined,
    },
    {
      id: 'enrichment',
      label: 'Enrichment Issues',
      badge: actualEnrichmentCount > 0 ? actualEnrichmentCount : undefined,
    },
  ]

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`Library Issues (${totalIssues})`} size="lg">
      <ModalContent>
        <Tabs
          tabs={tabs}
          activeTab={activeTab}
          onTabChange={setActiveTab}
          variant="underline"
          className="mb-4"
        />

        {/* Scan Issues Tab */}
        <TabPanel isActive={activeTab === 'scan'}>
          {scanLoading && (
            <div className="flex justify-center py-8">
              <Loading />
            </div>
          )}

          {scanError && (
            <Alert variant="error">{getErrorMessage(scanError, 'Failed to load scan errors')}</Alert>
          )}

          {scanData && (scanData.total_errors ?? 0) === 0 && (
            <div className={cn('text-center py-8', text.tertiary)}>No scan issues found</div>
          )}

          {scanData && (scanData.total_errors ?? 0) > 0 && scanData.by_category && (
            <div className="space-y-4">
              {Object.entries(scanData.by_category).map(([category, items]) => {
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
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M9 5l7 7-7 7"
                          />
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
                            const fileName =
                              item.file_path?.split('/').pop() || item.file_path || 'Unknown file'
                            const filePath = item.file_path || ''
                            return (
                              <li key={idx} className={cn('p-4 transition-colors', bg.hover.subtle)}>
                                <div className="space-y-3">
                                  <div className="flex items-start gap-3">
                                    {isWarning ? (
                                      <svg
                                        className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5"
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
                                    ) : (
                                      <svg
                                        className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5"
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
                                    )}
                                    <div className="flex-1 min-w-0 space-y-1">
                                      <p
                                        className={cn('text-sm font-medium break-words', text.primary)}
                                        title={filePath}
                                      >
                                        {fileName}
                                      </p>
                                      <p className={cn('text-xs font-mono break-all', text.tertiary)}>
                                        {filePath}
                                      </p>
                                      <div
                                        className={cn('flex flex-wrap gap-x-4 gap-y-1 text-xs', text.tertiary)}
                                      >
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
                                    <div
                                      className={`${isWarning ? 'bg-yellow-50 dark:bg-yellow-950 border-yellow-200 dark:border-yellow-800' : 'bg-red-50 dark:bg-red-950 border-red-200 dark:border-red-800'} border rounded-md p-3 ml-8`}
                                    >
                                      <p
                                        className={`text-sm leading-relaxed ${isWarning ? 'text-yellow-900 dark:text-yellow-100' : 'text-red-900 dark:text-red-100'}`}
                                      >
                                        {item.error_message}
                                      </p>
                                    </div>
                                  )}
                                </div>
                              </li>
                            )
                          })}
                        </ul>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </TabPanel>

        {/* Enrichment Issues Tab */}
        <TabPanel isActive={activeTab === 'enrichment'}>
          {enrichmentLoading && (
            <div className="flex justify-center py-8">
              <Loading />
            </div>
          )}

          {enrichmentError && (
            <Alert variant="error">
              {getErrorMessage(enrichmentError, 'Failed to load enrichment issues')}
            </Alert>
          )}

          {enrichmentData && (enrichmentData.total_errors ?? 0) === 0 && (
            <div className={cn('text-center py-8', text.tertiary)}>No enrichment issues found</div>
          )}

          {enrichmentData && (enrichmentData.total_errors ?? 0) > 0 && enrichmentData.by_stage && (
            <div className="space-y-4">
              {Object.entries(enrichmentData.by_stage).map(([stage, items]) => {
                const typedItems = items as InternalApiHandlersEnrichmentIssueDetail[]
                return (
                  <div key={stage} className="border rounded-lg overflow-hidden shadow-sm">
                    <button
                      className={`cursor-pointer w-full px-4 py-3 flex items-center justify-between ${getStageColor(stage)} border-b transition-colors hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500`}
                      onClick={() => toggleStage(stage)}
                      aria-expanded={expandedStage === stage}
                      aria-label={`${expandedStage === stage ? 'Collapse' : 'Expand'} ${formatStageName(stage)} stage with ${typedItems.length} items`}
                    >
                      <div className="flex items-center gap-3">
                        <svg
                          className={`w-4 h-4 transition-transform ${expandedStage === stage ? 'rotate-90' : ''}`}
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                          aria-hidden="true"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M9 5l7 7-7 7"
                          />
                        </svg>
                        <span className="font-semibold">{formatStageName(stage)}</span>
                      </div>
                      <span className="text-sm font-medium">{pluralize(typedItems.length, 'item')}</span>
                    </button>

                    {expandedStage === stage && (
                      <div className={cn(bg.elevated)}>
                        <ul className={cn('divide-y', border.secondary)}>
                          {typedItems.map((item, idx) => (
                            <li key={idx} className={cn('p-4 transition-colors', bg.hover.subtle)}>
                              <div className="space-y-3">
                                <div className="flex items-start gap-3">
                                  <svg
                                    className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5"
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
                                  <div className="flex-1 min-w-0 space-y-1">
                                    <p className={cn('text-sm font-medium break-words', text.primary)}>
                                      {item.title || 'Unknown'}
                                    </p>
                                    {item.file_path && (
                                      <p className={cn('text-xs font-mono break-all', text.tertiary)}>
                                        {item.file_path}
                                      </p>
                                    )}
                                    <div
                                      className={cn('flex flex-wrap gap-x-4 gap-y-1 text-xs', text.tertiary)}
                                    >
                                      <span className="capitalize">Type: {item.media_type}</span>
                                      {item.plugin_id && <span>Plugin: {item.plugin_id}</span>}
                                    </div>
                                  </div>
                                </div>
                                {item.error_message && (
                                  <div className="bg-red-50 dark:bg-red-950 border-red-200 dark:border-red-800 border rounded-md p-3 ml-8">
                                    <p className="text-sm leading-relaxed text-red-900 dark:text-red-100">
                                      {item.error_message}
                                    </p>
                                  </div>
                                )}
                              </div>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </TabPanel>
      </ModalContent>

      <ModalFooter className="justify-between">
        {activeTab === 'scan' && scanData && (scanData.total_errors ?? 0) > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {errorCount > 0 && warningCount > 0 && (
                <>
                  {pluralize(errorCount, 'error')} and {pluralize(warningCount, 'warning')}
                </>
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
                <Button onClick={handleRetry} variant="primary" isLoading={retryMutation.isPending}>
                  Retry Failed Files
                </Button>
              )}
            </div>
          </>
        ) : activeTab === 'enrichment' && enrichmentData && (enrichmentData.total_errors ?? 0) > 0 ? (
          <>
            <p className={cn('text-sm', text.secondary)}>
              {pluralize(enrichmentData.total_errors ?? 0, 'item')} failed during enrichment
            </p>
            <Button onClick={onClose} variant="secondary">
              Close
            </Button>
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

export { LibraryIssuesDialog }
export type { LibraryIssuesDialogProps } from './LibraryIssuesDialog.types'
