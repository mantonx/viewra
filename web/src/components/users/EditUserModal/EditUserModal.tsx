import { Modal, ModalContent } from '@/components/ui'
import { FormApiError, FormInput, FormModalFooter, FormToggle } from '@/components/ui/Form'
import { cn } from '@/lib/utils'
import { authFetch } from '@/lib/utils/authFetch'
import { getErrorMessage } from '@/lib/utils/error'
import { text } from '@/styles/semantic'
import { useForm } from '@tanstack/react-form'
import { useCallback, useEffect, useState } from 'react'
import { editUserSchema, type EditUserValues } from './EditUserModal.schema'
import type { EditUserModalProps } from './EditUserModal.types'

export const EditUserModal = ({
  isOpen,
  onClose,
  user,
  currentUserId,
  onSuccess,
}: EditUserModalProps) => {
  const [apiError, setApiError] = useState<string | null>(null)

  const isCurrentUser = user.id === currentUserId

  const getDefaultValues = useCallback(
    (): EditUserValues => ({
      displayName: user.display_name || '',
      isAdmin: user.is_admin || false,
    }),
    [user.display_name, user.is_admin]
  )

  const form = useForm({
    defaultValues: getDefaultValues(),
    validators: {
      onChange: editUserSchema,
    },
    onSubmit: async ({ value }) => {
      setApiError(null)

      try {
        const response = await authFetch(`/api/users/${user.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            display_name: value.displayName,
            is_admin: value.isAdmin,
          }),
        })

        if (!response.ok) {
          const data = await response.json().catch(() => ({}))
          throw new Error(data.error || 'Failed to update user')
        }

        const updatedUser = await response.json()
        onSuccess(updatedUser)
        onClose()
      } catch (error) {
        setApiError(getErrorMessage(error, 'Failed to update user'))
      }
    },
  })

  // Reset form when modal opens or user changes
  useEffect(() => {
    if (isOpen) {
      form.reset(getDefaultValues())
      setApiError(null)
    }
  }, [form, getDefaultValues, isOpen, user.id])

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Edit User: ${user.display_name || user.username}`}
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
            <form.Field name="displayName">
              {(field) => (
                <FormInput
                  field={field}
                  label="Display Name"
                  placeholder={user.username}
                  autoComplete="off"
                  autoFocus
                />
              )}
            </form.Field>

            <form.Field name="isAdmin">
              {(field) => (
                <div className="flex items-center gap-3">
                  <FormToggle field={field} label="Administrator" disabled={isCurrentUser} />
                  <div>
                    <span className={text.primary}>Administrator</span>
                    <p className={cn('text-xs', text.secondary)}>
                      Can manage users and system settings
                    </p>
                    {isCurrentUser && (
                      <p className={cn('text-xs mt-1', 'text-amber-600 dark:text-amber-400')}>
                        You cannot change your own admin status
                      </p>
                    )}
                  </div>
                </div>
              )}
            </form.Field>
          </div>
        </ModalContent>

        <FormModalFooter form={form} onCancel={onClose} submitLabel="Save Changes" />
      </form>
    </Modal>
  )
}
