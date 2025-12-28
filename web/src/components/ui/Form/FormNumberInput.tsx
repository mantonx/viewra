import { Input, type InputProps } from '@/components/ui/Input'
import type { DeepKeys } from '@tanstack/react-form'
import { getFieldError, type AnyFieldApi } from './Form.types'

type FormNumberInputProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
} & Omit<InputProps, 'value' | 'onChange' | 'onBlur' | 'error' | 'type'>

/**
 * Form-connected Number Input component.
 * Handles the string to number conversion required by HTML number inputs.
 * Automatically binds value, onChange, onBlur, and error display to TanStack Form field state.
 *
 * @example
 * <form.Field name="age">
 *   {(field) => <FormNumberInput field={field} label="Age" min={0} max={120} />}
 * </form.Field>
 */
export const FormNumberInput = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  ...props
}: FormNumberInputProps<TFormData, TName>) => {
  const error = getFieldError(field)

  return (
    <Input
      type="number"
      value={String(field.state.value ?? '')}
      onChange={(e) => {
        const value = e.target.value
        const numValue = value === '' ? 0 : Number(value)
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        field.handleChange(numValue as any)
      }}
      onBlur={field.handleBlur}
      error={error}
      {...props}
    />
  )
}
