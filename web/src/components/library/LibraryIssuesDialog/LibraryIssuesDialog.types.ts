export interface LibraryIssuesDialogProps {
  libraryId: number
  jobId?: number // Optional - if provided, enables retry functionality for scan errors
  isOpen: boolean
  onClose: () => void
  onRetrySuccess?: () => void
  /** Initial tab to display */
  initialTab?: 'scan' | 'enrichment'
  /** Counts passed in to avoid fetching just for badges */
  scanErrorCount?: number
  enrichmentErrorCount?: number
}
