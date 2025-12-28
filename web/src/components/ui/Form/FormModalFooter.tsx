import type { ReactNode } from 'react'
import { ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { FormSubmitButton } from './FormSubmitButton'
import type { AnyFormApi } from './Form.types'

type FormModalFooterProps = {
  /** TanStack Form instance */
  form: AnyFormApi
  /** Cancel button click handler */
  onCancel: () => void
  /** Submit button label (default: "Save Changes") */
  submitLabel?: ReactNode
  /** Cancel button label (default: "Cancel") */
  cancelLabel?: ReactNode
  /** Require form to be dirty before enabling submit (default: true) */
  requireDirty?: boolean
  /** Additional disabled condition for submit button */
  submitDisabled?: boolean
}

/**
 * Modal footer with Cancel and Submit buttons.
 * Submit button automatically subscribes to form state for loading/dirty state.
 *
 * @example
 * <FormModalFooter
 *   form={form}
 *   onCancel={onClose}
 *   submitLabel="Save Changes"
 * />
 */
export const FormModalFooter = ({
  form,
  onCancel,
  submitLabel = 'Save Changes',
  cancelLabel = 'Cancel',
  requireDirty = true,
  submitDisabled = false,
}: FormModalFooterProps) => {
  return (
    <ModalFooter>
      <Button variant="ghost" onClick={onCancel} type="button">
        {cancelLabel}
      </Button>
      <FormSubmitButton
        form={form}
        requireDirty={requireDirty}
        disabled={submitDisabled}
        variant="primary"
      >
        {submitLabel}
      </FormSubmitButton>
    </ModalFooter>
  )
}
