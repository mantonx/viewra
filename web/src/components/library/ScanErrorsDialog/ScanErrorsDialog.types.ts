export interface ScanErrorsDialogProps {
  libraryId: number
  jobId: number
  isOpen: boolean
  onClose: () => void
  onRetrySuccess?: () => void
}
