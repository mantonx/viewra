import type { ReactNode } from 'react'
import { Button, type ButtonProps } from '@/components/ui/Button'
import type { AnyFormApi } from './Form.types'

type FormSubmitButtonProps = Omit<ButtonProps, 'type' | 'isLoading' | 'disabled' | 'form'> & {
  /** TanStack Form instance */
  form: AnyFormApi
  /** Button label */
  children: ReactNode
  /** Require form to be dirty before enabling (default: true) */
  requireDirty?: boolean
  /** Additional disabled condition */
  disabled?: boolean
  /** Full width button (for standalone pages like login) */
  fullWidth?: boolean
}

/**
 * Submit button that automatically subscribes to form state.
 * Shows loading state during submission and disables when form is not dirty.
 *
 * @example
 * // In a modal
 * <FormSubmitButton form={form}>Save Changes</FormSubmitButton>
 *
 * // In a standalone page (full width)
 * <FormSubmitButton form={form} fullWidth requireDirty={false}>
 *   Sign In
 * </FormSubmitButton>
 */
export const FormSubmitButton = ({
  form,
  children,
  requireDirty = true,
  disabled = false,
  fullWidth = false,
  className,
  ...props
}: FormSubmitButtonProps) => {
  return (
    <form.Subscribe
      selector={(state: { isDirty: boolean; isSubmitting: boolean; canSubmit: boolean }) => [
        state.isDirty,
        state.isSubmitting,
        state.canSubmit,
      ]}
    >
      {([isDirty, isSubmitting, canSubmit]: [boolean, boolean, boolean]) => (
        <Button
          type="submit"
          isLoading={isSubmitting as boolean}
          disabled={disabled || !canSubmit || (requireDirty && !isDirty)}
          className={fullWidth ? `w-full ${className ?? ''}` : className}
          {...props}
        >
          {children}
        </Button>
      )}
    </form.Subscribe>
  )
}
