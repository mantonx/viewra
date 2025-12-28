import { useState } from 'react'
import { Input, type InputProps } from '@/components/ui/Input'
import { Eye, EyeOff } from 'lucide-react'
import type { DeepKeys } from '@tanstack/react-form'
import { getFieldError, type AnyFieldApi } from './Form.types'
import { cn } from '@/lib/utils'

type FormPasswordInputProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
} & Omit<InputProps, 'value' | 'onChange' | 'onBlur' | 'error' | 'type'>

/**
 * Form-connected Password Input component with visibility toggle.
 * Includes an eye icon button to show/hide the password.
 * Automatically binds value, onChange, onBlur, and error display to TanStack Form field state.
 *
 * @example
 * <form.Field name="password">
 *   {(field) => <FormPasswordInput field={field} label="Password" />}
 * </form.Field>
 */
export const FormPasswordInput = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  label,
  className,
  ...props
}: FormPasswordInputProps<TFormData, TName>) => {
  const [showPassword, setShowPassword] = useState(false)
  const error = getFieldError(field)

  // Position the toggle button based on whether there's a label
  const toggleButtonTop = label ? 'top-[38px]' : 'top-[10px]'

  return (
    <div className="relative w-full">
      <Input
        type={showPassword ? 'text' : 'password'}
        value={field.state.value as string}
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onChange={(e) => field.handleChange(e.target.value as any)}
        onBlur={field.handleBlur}
        error={error}
        label={label}
        className={cn('pr-10', className)}
        {...props}
      />
      <button
        type="button"
        onClick={() => setShowPassword(!showPassword)}
        className={cn(
          'absolute right-3 p-1 rounded transition-colors',
          'text-neutral-400 hover:text-neutral-600',
          'dark:text-neutral-500 dark:hover:text-neutral-300',
          'focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
          toggleButtonTop
        )}
        tabIndex={-1}
        aria-label={showPassword ? 'Hide password' : 'Show password'}
      >
        {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
      </button>
    </div>
  )
}
