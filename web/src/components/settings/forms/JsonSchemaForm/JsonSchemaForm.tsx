import Form from '@rjsf/core'
import validator from '@rjsf/validator-ajv8'
import type { RJSFSchema, UiSchema, RegistryFieldsType, RegistryWidgetsType } from '@rjsf/utils'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Button } from '@/components/ui'
import { useCallback, useMemo } from 'react'
// Note: parseDependsOn, shouldShowField, parsePropertyOrder are used in PluginSettingsForm
// to filter the schema before passing to JsonSchemaForm
import {
  TextWidget,
  PasswordWidget,
  CheckboxWidget,
  SelectWidget,
  MultiSelectWidget,
} from './widgets'
import { PluginRefField } from './fields'
import { hasPluginRef } from '@/lib/types/schema-actions'

// Custom field template to style form fields
// Note: description is a ReactElement (rendered DescriptionField), rawDescription is the string
const FieldTemplate = (props: {
  id: string
  label?: string
  help?: React.ReactNode
  rawHelp?: string
  required?: boolean
  description?: React.ReactElement
  rawDescription?: string
  errors?: React.ReactNode
  children: React.ReactNode
  hidden?: boolean
  schema?: { type?: string; 'x-viewra-plugin-ref'?: unknown }
}) => {
  const {
    id,
    label,
    rawHelp,
    required,
    rawDescription,
    errors,
    children,
    hidden,
    schema,
  } = props

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
      {/* Render description as span to avoid block-in-inline issues */}
      {rawDescription && !skipLabelAndDescription && (
        <span className={cn('block text-xs mt-1.5', text.tertiary)}>{rawDescription}</span>
      )}
      {rawHelp && (
        <span className={cn('block text-xs mt-1', text.secondary)}>{rawHelp}</span>
      )}
      {errors && <div className="text-xs text-red-500 mt-1">{errors}</div>}
    </div>
  )
}

// Custom object field template for nested objects
// Note: Property ordering and conditional visibility are handled in PluginSettingsForm
// by dynamically filtering the schema based on formData
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
// Note: RJSF passes submitButtonOptions inside ui:options (from getUiOptions transformation)
const SubmitButton = (props: { uiSchema?: UiSchema }) => {
  // RJSF wraps the options as { 'ui:options': { submitButtonOptions: ... } }
  const uiOptions = props.uiSchema?.['ui:options'] as
    | { submitButtonOptions?: { submitText?: string; norender?: boolean } }
    | undefined
  const options = uiOptions?.submitButtonOptions

  // Honor norender option to hide the submit button
  if (options?.norender) {
    return null
  }

  const submitText = options?.submitText || 'Save'
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
  MultiSelectWidget,
}

// Custom fields for specialized property types
const fields: RegistryFieldsType = {
  // PluginRefField renders a plugin selector with inline settings
  // Used when a property has x-viewra-plugin-ref extension
  PluginRefField,
}

export type JsonSchemaFormProps = {
  schema: RJSFSchema
  uiSchema?: UiSchema
  formData?: Record<string, unknown>
  onSubmit?: (data: Record<string, unknown>) => void
  onChange?: (data: Record<string, unknown>) => void
  disabled?: boolean
  className?: string
  hideSubmit?: boolean
  /** Render as div instead of form - use when embedding inside another form */
  asDiv?: boolean
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
  asDiv = false,
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

  // Build uiSchema with auto-detected widgets and fields
  const mergedUiSchema: UiSchema = useMemo(() => {
    const result: UiSchema = { ...uiSchema }

    // Auto-detect special fields from schema
    if (schema.properties) {
      Object.entries(schema.properties).forEach(([key, prop]) => {
        if (typeof prop !== 'object') {
          return
        }

        // Password fields -> PasswordWidget
        if (prop.format === 'password') {
          result[key] = {
            ...result[key],
            'ui:widget': 'PasswordWidget',
          }
        }

        // Plugin reference fields -> PluginRefField
        if (hasPluginRef(prop)) {
          result[key] = {
            ...result[key],
            'ui:field': 'PluginRefField',
          }
        }

        // Array with enum items -> MultiSelectWidget
        if (
          prop.type === 'array' &&
          prop.items &&
          typeof prop.items === 'object' &&
          'enum' in prop.items
        ) {
          result[key] = {
            ...result[key],
            'ui:widget': 'MultiSelectWidget',
          }
        }
      })
    }

    // Hide submit button if requested
    if (hideSubmit) {
      result['ui:submitButtonOptions'] = {
        norender: true,
      }
    }

    return result
  }, [schema.properties, uiSchema, hideSubmit])

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
        // Render as div instead of form when embedded in another form
        // This prevents nested <form> errors in React
        tagName={asDiv ? 'div' : undefined}
      />
    </div>
  )
}
