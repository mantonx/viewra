import { Input, type InputProps } from '@/components/ui/Input'
import type { DeepKeys } from '@tanstack/react-form'
import { getFieldError, type AnyFieldApi } from './Form.types'

type FormInputProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
} & Omit<InputProps, 'value' | 'onChange' | 'onBlur' | 'error'>

/**
 * Form-connected Input component.
 * Automatically binds value, onChange, onBlur, and error display to TanStack Form field state.
 *
 * @example
 * <form.Field name="username">
 *   {(field) => <FormInput field={field} label="Username" placeholder="Enter username" />}
 * </form.Field>
 */
export const FormInput = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  ...props
}: FormInputProps<TFormData, TName>) => {
  const error = getFieldError(field)

  return (
    <Input
      value={field.state.value as string}
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      onChange={(e) => field.handleChange(e.target.value as any)}
      onBlur={field.handleBlur}
      error={error}
      {...props}
    />
  )
}
