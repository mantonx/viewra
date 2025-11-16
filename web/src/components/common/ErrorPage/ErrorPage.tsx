import { Alert } from '@/components/ui'
import { getErrorMessage } from '@/lib/utils/error'
import type { ErrorPageProps } from './ErrorPage.types'

/**
 * Standardized error page component with consistent padding and styling.
 * Used across routes to display error states uniformly.
 */
export const ErrorPage = ({ error, context }: ErrorPageProps) => (
  <div className="p-8">
    <Alert variant="error">{getErrorMessage(error, `Error loading ${context}`)}</Alert>
  </div>
)
