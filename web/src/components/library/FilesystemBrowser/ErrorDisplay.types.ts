export interface ErrorDisplayProps {
  error: Error | unknown
  onRetry: () => void
  onNavigateUp?: () => void
  canNavigateUp: boolean
}
