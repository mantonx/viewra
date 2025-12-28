import { useState } from 'react'
import { useForm } from '@tanstack/react-form'
import { authFetch } from '@/lib/utils/authFetch'
import { getErrorMessage } from '@/lib/utils/error'
import { FormInput, FormPasswordInput, FormToggle, FormApiError, FormModalFooter } from '@/components/ui/Form'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import type { CreateUserFormProps } from './CreateUserForm.types'
import { createUserFormSchema, type CreateUserFormValues } from './CreateUserForm.schema'

const DEFAULT_VALUES: CreateUserFormValues = {
  username: '',
  displayName: '',
  password: '',
  isAdmin: false,
}

export const CreateUserForm = ({ onSuccess, onCancel }: CreateUserFormProps) => {
  const [apiError, setApiError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: DEFAULT_VALUES,
    validators: {
      onChange: createUserFormSchema,
    },
    onSubmit: async ({ value }) => {
      setApiError(null)

      try {
        const response = await authFetch('/api/users', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: value.username,
            display_name: value.displayName || value.username,
            password: value.password,
            is_admin: value.isAdmin,
          }),
        })

        if (!response.ok) {
          const data = await response.json().catch(() => ({}))
          throw new Error(data.error || 'Failed to create user')
        }

        const newUser = await response.json()
        onSuccess(newUser)
      } catch (error) {
        setApiError(getErrorMessage(error, 'Failed to create user'))
      }
    },
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        form.handleSubmit()
      }}
    >
      <div className="p-6 space-y-4">
        <FormApiError error={apiError} />

        <form.Field name="username">
          {(field) => (
            <FormInput
              field={field}
              label="Username"
              autoComplete="off"
              autoFocus
              helperText="Used for signing in"
            />
          )}
        </form.Field>

        <form.Field name="displayName">
          {(field) => (
            <FormInput
              field={field}
              label="Display Name"
              placeholder="Optional"
              autoComplete="off"
            />
          )}
        </form.Field>

        <form.Field name="password">
          {(field) => (
            <FormPasswordInput
              field={field}
              label="Password"
              autoComplete="new-password"
              helperText="At least 8 characters"
            />
          )}
        </form.Field>

        <form.Field name="isAdmin">
          {(field) => (
            <div className="flex items-center gap-3">
              <FormToggle field={field} label="Administrator" />
              <div>
                <span className={text.primary}>Administrator</span>
                <p className={cn('text-xs', text.secondary)}>
                  Can manage users and system settings
                </p>
              </div>
            </div>
          )}
        </form.Field>
      </div>

      <FormModalFooter form={form} onCancel={onCancel} submitLabel="Create User" />
    </form>
  )
}
