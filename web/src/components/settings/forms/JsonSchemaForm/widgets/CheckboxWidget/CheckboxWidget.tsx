import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
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
    <label
      htmlFor={id}
      className={cn(
        'flex items-start gap-3 cursor-pointer',
        (disabled || readonly) && 'opacity-50 cursor-not-allowed'
      )}
    >
      <input
        id={id}
        type="checkbox"
        checked={value ?? false}
        onChange={(e) => onChange(e.target.checked)}
        disabled={disabled || readonly}
        className={cn(
          'mt-0.5 h-4 w-4 rounded border-neutral-300 dark:border-neutral-600',
          'text-primary-600 focus:ring-primary-500',
          'dark:bg-neutral-800'
        )}
      />
      <div>
        <span className={cn('text-sm font-medium', text.primary)}>{label}</span>
        {schema?.description && (
          <p className={cn('text-xs mt-0.5', text.tertiary)}>{schema.description}</p>
        )}
      </div>
    </label>
  )
}
