export type IssueTab = 'errors' | 'warnings' | 'enrichment'

export interface ScanErrorsDialogProps {
  libraryId: number
  jobId?: number // Optional - if not provided, shows library-level persistent issues
  isOpen: boolean
  onClose: () => void
  onRetrySuccess?: () => void
  initialTab?: IssueTab // Optional - which tab to open initially
}
