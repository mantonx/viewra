import { useState } from 'react'
import { useForm } from '@tanstack/react-form'
import { useAuth } from '@/contexts'
import { FormInput, FormPasswordInput, FormApiError, FormSubmitButton } from '@/components/ui/Form'
import { getErrorMessage } from '@/lib/utils/error'
import type { SetupFormProps } from './SetupForm.types'
import { setupFormSchema, type SetupFormValues } from './SetupForm.schema'

const DEFAULT_VALUES: SetupFormValues = {
  username: '',
  displayName: '',
  password: '',
  confirmPassword: '',
}

export const SetupForm = ({ onSuccess, variant = 'default' }: SetupFormProps) => {
  const { login } = useAuth()
  const [apiError, setApiError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: DEFAULT_VALUES,
    validators: {
      onChange: setupFormSchema,
    },
    onSubmit: async ({ value }) => {
      setApiError(null)

      try {
        // Create admin account
        const response = await fetch('/api/auth/setup', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: value.username,
            password: value.password,
            display_name: value.displayName || value.username,
          }),
        })

        if (!response.ok) {
          const data = await response.json().catch(() => ({ error: 'Setup failed' }))
          throw new Error(data.error || 'Setup failed')
        }

        // Auto-login after setup
        await login(value.username, value.password)
        onSuccess()
      } catch (error) {
        setApiError(getErrorMessage(error, 'Setup failed'))
      }
    },
  })

  const inputVariant = variant === 'glass' ? 'glass' : undefined

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        form.handleSubmit()
      }}
      className="auth-form-enter space-y-5"
    >
      <FormApiError error={apiError} className="mb-6" />

      <form.Field name="username">
        {(field) => (
          <FormInput
            field={field}
            label="Username"
            autoComplete="username"
            autoFocus
            variant={inputVariant}
            helperText="This will be used to sign in"
          />
        )}
      </form.Field>

      <form.Field name="displayName">
        {(field) => (
          <FormInput
            field={field}
            label="Display Name"
            placeholder={form.getFieldValue('username') || 'Optional'}
            autoComplete="name"
            variant={inputVariant}
          />
        )}
      </form.Field>

      <form.Field name="password">
        {(field) => (
          <FormPasswordInput
            field={field}
            label="Password"
            autoComplete="new-password"
            variant={inputVariant}
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
            variant={inputVariant}
          />
        )}
      </form.Field>

      <FormSubmitButton
        form={form}
        requireDirty={false}
        fullWidth
        size="lg"
        className="mt-4 rounded-lg font-semibold"
      >
        Create Account
      </FormSubmitButton>
    </form>
  )
}
