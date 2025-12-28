import { useState } from 'react'
import { useForm } from '@tanstack/react-form'
import { Card, CardHeader, CardContent } from '@/components/ui'
import { FormPasswordInput, FormApiError, FormSubmitButton } from '@/components/ui/Form'
import { authFetch } from '@/lib/utils/authFetch'
import { useToast } from '@/lib/hooks/useToast'
import { getErrorMessage } from '@/lib/utils/error'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ChangePasswordProps } from './ChangePassword.types'
import { changePasswordSchema, type ChangePasswordFormValues } from './ChangePassword.schema'

const DEFAULT_VALUES: ChangePasswordFormValues = {
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
}

export const ChangePassword = ({ className }: ChangePasswordProps) => {
  const toast = useToast()
  const [apiError, setApiError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: DEFAULT_VALUES,
    validators: {
      onChange: changePasswordSchema,
    },
    onSubmit: async ({ value }) => {
      setApiError(null)

      try {
        const response = await authFetch('/api/auth/password', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            current_password: value.currentPassword,
            new_password: value.newPassword,
          }),
        })

        if (!response.ok) {
          const data = await response.json().catch(() => ({}))
          throw new Error(data.error || 'Failed to change password')
        }

        toast.success('Password changed successfully')
        form.reset()
      } catch (error) {
        setApiError(getErrorMessage(error, 'Failed to change password'))
      }
    },
  })

  return (
    <Card className={className}>
      <CardHeader>
        <h2 className={cn('text-lg font-semibold', text.primary)}>Change Password</h2>
        <p className={cn('text-sm mt-1', text.secondary)}>
          Update your password to keep your account secure
        </p>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            e.stopPropagation()
            form.handleSubmit()
          }}
          className="space-y-4 max-w-md"
        >
          <FormApiError error={apiError} />

          <form.Field name="currentPassword">
            {(field) => (
              <FormPasswordInput
                field={field}
                label="Current Password"
                autoComplete="current-password"
              />
            )}
          </form.Field>

          <form.Field name="newPassword">
            {(field) => (
              <FormPasswordInput
                field={field}
                label="New Password"
                autoComplete="new-password"
                helperText="At least 8 characters"
              />
            )}
          </form.Field>

          <form.Field name="confirmPassword">
            {(field) => (
              <FormPasswordInput
                field={field}
                label="Confirm New Password"
                autoComplete="new-password"
              />
            )}
          </form.Field>

          <FormSubmitButton form={form} requireDirty={false}>
            Change Password
          </FormSubmitButton>
        </form>
      </CardContent>
    </Card>
  )
}
