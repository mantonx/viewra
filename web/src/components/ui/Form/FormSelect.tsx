import { Select, type SelectProps } from '@/components/ui/Select'
import type { DeepKeys } from '@tanstack/react-form'
import { getFieldError, type AnyFieldApi } from './Form.types'

type FormSelectProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
} & Omit<SelectProps, 'value' | 'onChange' | 'onBlur' | 'error'>

/**
 * Form-connected Select component.
 * Automatically binds value, onChange, onBlur, and error display to TanStack Form field state.
 *
 * @example
 * <form.Field name="country">
 *   {(field) => (
 *     <FormSelect
 *       field={field}
 *       label="Country"
 *       options={[
 *         { value: 'us', label: 'United States' },
 *         { value: 'uk', label: 'United Kingdom' },
 *       ]}
 *     />
 *   )}
 * </form.Field>
 */
export const FormSelect = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  ...props
}: FormSelectProps<TFormData, TName>) => {
  const error = getFieldError(field)

  return (
    <Select
      value={field.state.value as string}
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      onChange={(e) => field.handleChange(e.target.value as any)}
      onBlur={field.handleBlur}
      error={error}
      {...props}
    />
  )
}
