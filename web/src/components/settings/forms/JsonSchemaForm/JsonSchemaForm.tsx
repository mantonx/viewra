import Form from '@rjsf/core'
import validator from '@rjsf/validator-ajv8'
import type { RJSFSchema, UiSchema, RegistryFieldsType, RegistryWidgetsType } from '@rjsf/utils'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Button } from '@/components/ui'
import { useCallback } from 'react'
import { TextWidget, PasswordWidget, CheckboxWidget, SelectWidget } from './widgets'

// Custom field template to style form fields
const FieldTemplate = (props: {
  id: string
  label?: string
  help?: string
  required?: boolean
  description?: string
  errors?: React.ReactNode
  children: React.ReactNode
  hidden?: boolean
  schema?: { type?: string }
}) => {
  const { id, label, help, required, description, errors, children, hidden, schema } = props

  if (hidden) {
    return <div className="hidden">{children}</div>
  }

  // Don't render label/description for these types - they handle it themselves
  const isObject = schema?.type === 'object'
  const isBoolean = schema?.type === 'boolean'
  const skipLabelAndDescription = isObject || isBoolean

  return (
    <div className="mb-4">
      {label && !skipLabelAndDescription && (
        <label htmlFor={id} className={cn('block text-sm font-medium mb-2', text.primary)}>
          {label}
          {required && <span className="text-red-500 ml-1">*</span>}
        </label>
      )}
      {children}
      {description && !skipLabelAndDescription && (
        <p className={cn('text-xs mt-1.5', text.tertiary)}>{description}</p>
      )}
      {help && <p className={cn('text-xs mt-1', text.secondary)}>{help}</p>}
      {errors && <div className="text-xs text-red-500 mt-1">{errors}</div>}
    </div>
  )
}

// Custom object field template for nested objects
const ObjectFieldTemplate = (props: {
  title: string
  description?: string
  properties: Array<{ content: React.ReactNode; name: string }>
  fieldPathId: string[]
}) => {
  const { title, description, properties, fieldPathId } = props
  // Root object has empty fieldPathId array
  const isRoot = !fieldPathId || fieldPathId.length === 0

  if (isRoot) {
    // Root renders children with spacing, first child gets no top border
    return (
      <div className="space-y-0">
        {properties.map((prop, index) => (
          <div
            key={prop.name}
            className={cn(
              index > 0 && 'mt-4 pt-4 border-t border-neutral-100 dark:border-neutral-800'
            )}
          >
            {prop.content}
          </div>
        ))}
      </div>
    )
  }

  // Nested objects render title + children
  return (
    <div>
      {title && <h3 className={cn('text-sm font-medium mb-3', text.primary)}>{title}</h3>}
      {description && <p className={cn('text-xs mb-3', text.tertiary)}>{description}</p>}
      <div>
        {properties.map((prop) => (
          <div key={prop.name}>{prop.content}</div>
        ))}
      </div>
    </div>
  )
}

// Custom submit button template
const SubmitButton = (props: { uiSchema?: UiSchema }) => {
  const submitText = props.uiSchema?.['ui:submitButtonOptions']?.submitText || 'Save'
  return (
    <Button type="submit" className="mt-4">
      {submitText}
    </Button>
  )
}

// Custom widgets
const widgets: RegistryWidgetsType = {
  TextWidget,
  PasswordWidget,
  CheckboxWidget,
  SelectWidget,
}

// Custom fields (empty for now, can add custom field types)
const fields: RegistryFieldsType = {}

export type JsonSchemaFormProps = {
  schema: RJSFSchema
  uiSchema?: UiSchema
  formData?: Record<string, unknown>
  onSubmit?: (data: Record<string, unknown>) => void
  onChange?: (data: Record<string, unknown>) => void
  disabled?: boolean
  className?: string
  hideSubmit?: boolean
}

export const JsonSchemaForm = ({
  schema,
  uiSchema,
  formData,
  onSubmit,
  onChange,
  disabled,
  className,
  hideSubmit = false,
}: JsonSchemaFormProps) => {
  const handleSubmit = useCallback(
    ({ formData: data }: { formData?: Record<string, unknown> }) => {
      if (onSubmit && data) {
        onSubmit(data)
      }
    },
    [onSubmit]
  )

  const handleChange = useCallback(
    ({ formData: data }: { formData?: Record<string, unknown> }) => {
      if (onChange && data) {
        onChange(data)
      }
    },
    [onChange]
  )

  // Build uiSchema with password widget for password fields
  const mergedUiSchema: UiSchema = {
    ...uiSchema,
  }

  // Auto-detect password fields from schema
  if (schema.properties) {
    Object.entries(schema.properties).forEach(([key, prop]) => {
      if (typeof prop === 'object' && prop.format === 'password') {
        mergedUiSchema[key] = {
          ...mergedUiSchema[key],
          'ui:widget': 'PasswordWidget',
        }
      }
    })
  }

  // Hide submit button if requested
  if (hideSubmit) {
    mergedUiSchema['ui:submitButtonOptions'] = {
      norender: true,
    }
  }

  return (
    <div className={cn('json-schema-form', className)}>
      <Form
        schema={schema}
        uiSchema={mergedUiSchema}
        formData={formData}
        validator={validator}
        onSubmit={handleSubmit}
        onChange={handleChange}
        disabled={disabled}
        widgets={widgets}
        fields={fields}
        templates={{
          FieldTemplate,
          ObjectFieldTemplate,
          ButtonTemplates: { SubmitButton },
        }}
        // Disable HTML5 validation, use AJV instead
        noHtml5Validate
      />
    </div>
  )
}
