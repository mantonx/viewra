import { Alert } from '@/components/ui/Alert'
import { cn } from '@/lib/utils'

type FormApiErrorProps = {
  /** Error message to display (null/undefined = hidden) */
  error: string | null | undefined
  /** Additional class names */
  className?: string
}

/**
 * Displays an API error alert at the top of a form.
 * Renders nothing when error is null/undefined.
 *
 * @example
 * <FormApiError error={apiError} />
 *
 * // With custom spacing
 * <FormApiError error={apiError} className="mb-6" />
 */
export const FormApiError = ({ error, className }: FormApiErrorProps) => {
  if (!error) {
    return null
  }

  return (
    <Alert variant="error" className={cn('mb-4', className)}>
      {error}
    </Alert>
  )
}
