import { Alert } from '@/components/ui/Alert/Alert'
import { Button } from '@/components/ui/Button/Button'
import type { ErrorDisplayProps } from './ErrorDisplay.types'

const ErrorDisplay = ({ error, onRetry, onNavigateUp, canNavigateUp }: ErrorDisplayProps) => {
  const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred'

  return (
    <div className="mb-4">
      <Alert variant="error" className="mb-2">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <p className="font-medium">Failed to load directory</p>
            <p className="text-sm mt-1">{errorMessage}</p>
          </div>
        </div>
      </Alert>
      <div className="flex gap-2">
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Try Again
        </Button>
        {canNavigateUp && onNavigateUp && (
          <Button variant="secondary" size="sm" onClick={onNavigateUp}>
            ← Go to Parent Directory
          </Button>
        )}
      </div>
    </div>
  )
}

export { ErrorDisplay }
