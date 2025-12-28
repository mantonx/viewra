import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { FormInput, FormNumberInput, FormSelect, FormToggle } from '@/components/ui/Form'
import { SourceBadge, RestartBadge } from '../badges'
import type { DynamicSettingRowProps } from './DynamicSettingRow.types'

/**
 * Dynamic setting row that renders the appropriate input based on setting type.
 * Supports boolean toggles, select dropdowns, number inputs, and text inputs.
 *
 * @example
 * <DynamicSettingRow
 *   fieldKey="transcoding.threads"
 *   type="int"
 *   label="Transcode Threads"
 *   description="Number of threads to use for transcoding"
 *   form={form}
 *   isChanged={hasFieldChanged('transcoding.threads')}
 *   sourceBadge={{ source: 'database' }}
 *   showRestartBadge
 * />
 */
export const DynamicSettingRow = ({
  fieldKey,
  type = 'string',
  label,
  description,
  options,
  form,
  isChanged = false,
  disabled = false,
  sourceBadge,
  showRestartBadge = false,
  suffix,
  className,
}: DynamicSettingRowProps) => {
  const displayLabel = label || fieldKey

  // Wrapper classes for change highlighting
  const wrapperClasses = cn(
    'py-3 transition-all duration-150',
    isChanged && 'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500',
    className
  )

  // Label row with badges
  const labelRow = (
    <div className="flex items-center gap-2 flex-wrap mb-1">
      <span className={cn('text-sm font-medium', text.primary)}>
        {displayLabel}
      </span>
      {sourceBadge && <SourceBadge {...sourceBadge} />}
      {showRestartBadge && <RestartBadge />}
    </div>
  )

  // Description text
  const descriptionText = description && (
    <p className={cn('text-xs', text.tertiary)}>{description}</p>
  )

  // Boolean toggle - horizontal layout
  if (type === 'bool') {
    return (
      <div className={wrapperClasses}>
        <div className="flex items-start gap-4">
          <div className="flex-1">
            {labelRow}
            {descriptionText}
          </div>
          <form.Field name={fieldKey}>
            {(field) => (
              <FormToggle
                field={field}
                label={displayLabel}
                disabled={disabled}
              />
            )}
          </form.Field>
        </div>
        {suffix}
      </div>
    )
  }

  // Select dropdown - vertical layout
  if (options && options.length > 0) {
    return (
      <div className={wrapperClasses}>
        {labelRow}
        <form.Field name={fieldKey}>
          {(field) => (
            <FormSelect
              field={field}
              options={options}
              disabled={disabled}
            />
          )}
        </form.Field>
        {descriptionText && <div className="mt-1.5">{descriptionText}</div>}
        {suffix}
      </div>
    )
  }

  // Number input - vertical layout
  if (type === 'int') {
    return (
      <div className={wrapperClasses}>
        {labelRow}
        <form.Field name={fieldKey}>
          {(field) => (
            <FormNumberInput
              field={field}
              disabled={disabled}
              className="max-w-xs"
            />
          )}
        </form.Field>
        {descriptionText && <div className="mt-1.5">{descriptionText}</div>}
        {suffix}
      </div>
    )
  }

  // Default: string input - vertical layout
  return (
    <div className={wrapperClasses}>
      {labelRow}
      <form.Field name={fieldKey}>
        {(field) => (
          <FormInput
            field={field}
            disabled={disabled}
          />
        )}
      </form.Field>
      {descriptionText && <div className="mt-1.5">{descriptionText}</div>}
      {suffix}
    </div>
  )
}
