import { Button } from '@/components/ui'
import { FormInput } from '@/components/ui/Form'
import { useForm } from '@tanstack/react-form'
import { Download, Loader2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { z } from 'zod'
import type { CreateFormProps } from './CreateForm.types'

export const CreateForm = ({
  schema,
  formData,
  isSubmitting,
  onChange,
  onSubmit,
}: CreateFormProps) => {
  // Build Zod schema dynamically from the action schema
  const zodSchema = useMemo(() => {
    if (!schema?.properties) {
      return z.object({})
    }

    const shape: Record<string, z.ZodTypeAny> = {}
    for (const [key, prop] of Object.entries(schema.properties)) {
      let fieldSchema = z.string()
      if (schema.required?.includes(key)) {
        fieldSchema = fieldSchema.min(1, `${prop.title || key} is required`)
      }
      shape[key] = fieldSchema
    }
    return z.object(shape)
  }, [schema])

  // Build default values from schema properties
  const defaultValues = useMemo(() => {
    if (!schema?.properties) {
      return {}
    }

    const values: Record<string, string> = {}
    for (const key of Object.keys(schema.properties)) {
      values[key] = formData[key] || ''
    }
    return values
  }, [schema, formData])

  const form = useForm({
    defaultValues,
    validators: {
      onChange: zodSchema,
    },
    onSubmit: () => {
      onSubmit()
    },
  })

  // Sync external formData changes to form state
  useEffect(() => {
    for (const [key, value] of Object.entries(formData)) {
      const currentValue = form.getFieldValue(key as never)
      if (currentValue !== value) {
        form.setFieldValue(key as never, value as never)
      }
    }
  }, [form, formData])

  // Subscribe to form changes and sync back to parent
  useEffect(() => {
    const unsubscribe = form.store.subscribe(() => {
      const values = form.state.values
      for (const [key, value] of Object.entries(values)) {
        if (formData[key] !== value) {
          onChange(key, value as string)
        }
      }
    })
    return unsubscribe
  }, [form, formData, onChange])

  if (!schema?.properties) {
    return null
  }

  const propertyEntries = Object.entries(schema.properties)

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        form.handleSubmit()
      }}
      className="flex gap-2"
    >
      {propertyEntries.map(([key, prop]) => (
        <div key={key} className="flex-1">
          <form.Field name={key as never}>
            {(field) => (
              <FormInput
                field={field}
                placeholder={prop.description || prop.title || key}
                disabled={isSubmitting}
              />
            )}
          </form.Field>
        </div>
      ))}
      <form.Subscribe selector={(state) => [state.canSubmit]}>
        {([canSubmit]) => (
          <Button type="submit" disabled={isSubmitting || !canSubmit}>
            {isSubmitting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Download className="w-4 h-4" />
            )}
          </Button>
        )}
      </form.Subscribe>
    </form>
  )
}

CreateForm.displayName = 'CreateForm'
