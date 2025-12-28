import { Toggle, type ToggleProps } from '@/components/ui/Toggle'
import type { DeepKeys } from '@tanstack/react-form'
import type { AnyFieldApi } from './Form.types'

type FormToggleProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
} & Omit<ToggleProps, 'enabled' | 'onChange'>

/**
 * Form-connected Toggle component.
 * Automatically binds enabled state and onChange to TanStack Form field state.
 *
 * @example
 * <form.Field name="notifications">
 *   {(field) => <FormToggle field={field} label="Enable notifications" />}
 * </form.Field>
 */
export const FormToggle = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  ...props
}: FormToggleProps<TFormData, TName>) => {
  return (
    <Toggle
      enabled={field.state.value as boolean}
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      onChange={(val) => field.handleChange(val as any)}
      {...props}
    />
  )
}
