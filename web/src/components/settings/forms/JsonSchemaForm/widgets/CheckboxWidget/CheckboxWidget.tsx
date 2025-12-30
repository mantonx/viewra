import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Toggle } from '@/components/ui'
import type { CheckboxWidgetProps } from './CheckboxWidget.types'

export const CheckboxWidget = ({
  id,
  value,
  onChange,
  label,
  disabled,
  readonly,
  schema,
}: CheckboxWidgetProps) => {
  return (
    <div
      className={cn(
        'flex items-start gap-3',
        (disabled || readonly) && 'opacity-50'
      )}
    >
      <Toggle
        enabled={value ?? false}
        onChange={(enabled) => onChange(enabled)}
        label={label ?? ''}
        disabled={disabled || readonly}
      />
      <div>
        <label
          htmlFor={id}
          className={cn(
            'text-sm font-medium cursor-pointer',
            text.primary,
            (disabled || readonly) && 'cursor-not-allowed'
          )}
          onClick={() => !disabled && !readonly && onChange(!value)}
        >
          {label}
        </label>
        {schema?.description && (
          <p className={cn('text-xs mt-0.5', text.tertiary)}>{schema.description}</p>
        )}
      </div>
    </div>
  )
}
