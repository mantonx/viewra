/**
 * Types for CreateForm component.
 */

import type { ActionSchema } from '@/lib/types/schema-actions'

export interface CreateFormProps {
  /** Form schema */
  schema?: ActionSchema
  /** Current form data */
  formData: Record<string, string>
  /** Whether submitting */
  isSubmitting: boolean
  /** Called when field changes */
  onChange: (field: string, value: string) => void
  /** Called on submit */
  onSubmit: () => void
}
