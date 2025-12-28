import type { DeepKeys, DeepValue, FieldApi } from '@tanstack/react-form'

/**
 * Simplified FieldApi type for form field adapters.
 * Uses `any` for validator type parameters since adapters don't need that level of type detail.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type AnyFieldApi<TFormData, TName extends DeepKeys<TFormData>> = FieldApi<
  TFormData,
  TName,
  DeepValue<TFormData, TName>,
  any, any, any, any, any, any, any, any, any, // Field validators
  any, any, any, any, any, any, any, any, any, any, // Form validators
  any // Submit meta
>

/**
 * Simplified form type for components that need to access form state (Subscribe, etc.)
 * but don't need full type safety on validators.
 *
 * We use `any` here because TanStack Form's generics are too complex to work with directly.
 * The tradeoff is we lose type checking on the form prop, but we gain usability.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type AnyFormApi = any

/**
 * Base props for all form field adapter components.
 * Provides the TanStack Form field API for connecting UI components to form state.
 */
export type FieldComponentProps<
  TFormData,
  TName extends DeepKeys<TFormData>,
> = {
  field: AnyFieldApi<TFormData, TName>
}

/**
 * Helper to extract the first error message from field state.
 * Returns undefined if no errors exist.
 */
export const getFieldError = <TFormData, TName extends DeepKeys<TFormData>>(
  field: AnyFieldApi<TFormData, TName>
): string | undefined => {
  const errors = field.state.meta.errors
  if (errors.length === 0) return undefined

  const firstError = errors[0]
  if (typeof firstError === 'string') return firstError
  if (firstError && typeof firstError === 'object' && 'message' in firstError) {
    return String(firstError.message)
  }
  return String(firstError)
}
