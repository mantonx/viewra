import { useState, useEffect } from 'react'
import { useForm } from '@tanstack/react-form'
import { Modal, ModalContent } from '@/components/ui'
import { authFetch } from '@/lib/utils/authFetch'
import { getErrorMessage } from '@/lib/utils/error'
import { FormPasswordInput, FormApiError, FormModalFooter } from '@/components/ui/Form'
import type { ResetPasswordModalProps } from './ResetPasswordModal.types'
import { resetPasswordSchema, type ResetPasswordValues } from './ResetPasswordModal.schema'

const DEFAULT_VALUES: ResetPasswordValues = {
  newPassword: '',
  confirmPassword: '',
}

export const ResetPasswordModal = ({
  isOpen,
  onClose,
  userId,
  username,
  onSuccess,
}: ResetPasswordModalProps) => {
  const [apiError, setApiError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: DEFAULT_VALUES,
    validators: {
      onChange: resetPasswordSchema,
    },
    onSubmit: async ({ value }) => {
      setApiError(null)

      try {
        const response = await authFetch(`/api/users/${userId}/reset-password`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ new_password: value.newPassword }),
        })

        if (!response.ok) {
          const data = await response.json().catch(() => ({}))
          throw new Error(data.error || 'Failed to reset password')
        }

        onSuccess()
        onClose()
      } catch (error) {
        setApiError(getErrorMessage(error, 'Failed to reset password'))
      }
    },
  })

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      form.reset()
      setApiError(null)
    }
  }, [isOpen, form])

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Reset Password for ${username}`}
      size="sm"
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          e.stopPropagation()
          form.handleSubmit()
        }}
      >
        <ModalContent>
          <FormApiError error={apiError} />

          <div className="space-y-4">
            <form.Field name="newPassword">
              {(field) => (
                <FormPasswordInput
                  field={field}
                  label="New Password"
                  autoComplete="new-password"
                  autoFocus
                  helperText="At least 8 characters"
                />
              )}
            </form.Field>

            <form.Field name="confirmPassword">
              {(field) => (
                <FormPasswordInput
                  field={field}
                  label="Confirm Password"
                  autoComplete="new-password"
                />
              )}
            </form.Field>
          </div>
        </ModalContent>

        <FormModalFooter form={form} onCancel={onClose} submitLabel="Reset Password" />
      </form>
    </Modal>
  )
}
