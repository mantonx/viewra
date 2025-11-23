export interface ScanErrorsDialogProps {
  libraryId: number
  jobId?: number // Optional - if not provided, shows library-level persistent issues
  isOpen: boolean
  onClose: () => void
  onRetrySuccess?: () => void
}
