import { useState } from 'react'
import { useForm } from '@tanstack/react-form'
import { Button } from '@/components/ui/Button/Button'
import { FormInput } from '@/components/ui/Form'
import type { PathInputProps } from './PathInput.types'
import { pathInputSchema, type PathInputValues } from './PathInput.schema'

const DEFAULT_VALUES: PathInputValues = {
  path: '',
}

const PathInput = ({ onNavigate, isLoading }: PathInputProps) => {
  const [showInput, setShowInput] = useState(false)

  const form = useForm({
    defaultValues: DEFAULT_VALUES,
    validators: {
      onChange: pathInputSchema,
    },
    onSubmit: ({ value }) => {
      onNavigate(value.path.trim())
      form.reset()
      setShowInput(false)
    },
  })

  const handleCancel = () => {
    setShowInput(false)
    form.reset()
  }

  if (!showInput) {
    return (
      <div className="mb-4">
        <button
          onClick={() => setShowInput(true)}
          className="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline"
        >
          Jump to path...
        </button>
      </div>
    )
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        form.handleSubmit()
      }}
      className="mb-4"
    >
      <div className="flex gap-2">
        <div className="flex-1">
          <form.Field name="path">
            {(field) => (
              <FormInput
                field={field}
                placeholder="Type a path (e.g., /home/user/Videos)"
                disabled={isLoading}
                autoFocus
              />
            )}
          </form.Field>
        </div>
        <form.Subscribe selector={(state) => [state.canSubmit, state.isSubmitting]}>
          {([canSubmit, isSubmitting]) => (
            <Button
              type="submit"
              variant="secondary"
              size="sm"
              disabled={isLoading || !canSubmit || isSubmitting}
            >
              Go
            </Button>
          )}
        </form.Subscribe>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={handleCancel}
        >
          Cancel
        </Button>
      </div>
    </form>
  )
}

export { PathInput }
