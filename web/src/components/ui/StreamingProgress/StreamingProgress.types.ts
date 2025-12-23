/**
 * Types for StreamingProgress component.
 */

export interface StreamingProgressProps {
  /** Current status message */
  status?: string
  /** Progress percentage (0-100), or undefined for indeterminate */
  percent?: number
  /** Whether the operation is complete */
  done?: boolean
  /** Error message if failed */
  error?: string
  /** Called when cancel button is clicked */
  onCancel?: () => void
  /** Variant - inline is more compact */
  variant?: 'inline' | 'standalone'
  /** Additional class names */
  className?: string
}
