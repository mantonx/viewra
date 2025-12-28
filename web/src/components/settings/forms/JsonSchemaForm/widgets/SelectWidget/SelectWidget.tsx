import { cn } from '@/lib/utils'
import type { SelectWidgetProps } from './SelectWidget.types'

export const SelectWidget = ({
  id,
  value,
  onChange,
  options,
  disabled,
  readonly,
  placeholder,
}: SelectWidgetProps) => {
  const enumOptions = options.enumOptions ?? []

  return (
    <select
      id={id}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled || readonly}
      className={cn(
        'w-full px-3 py-2 rounded-lg transition-all duration-150',
        // Light mode
        'bg-white text-neutral-900',
        'border border-neutral-200/50',
        'hover:border-neutral-300',
        // Dark mode - glass effect
        'dark:bg-white/5 dark:text-neutral-50',
        'dark:border-white/10',
        'dark:hover:border-white/20',
        // Focus states
        'focus:outline-none focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500',
        // Disabled
        'disabled:bg-neutral-50 dark:disabled:bg-neutral-900/50',
        'disabled:text-neutral-500 dark:disabled:text-neutral-500',
        'disabled:cursor-not-allowed'
      )}
    >
      {placeholder && (
        <option value="" disabled className="bg-white dark:bg-neutral-800">
          {placeholder}
        </option>
      )}
      {enumOptions.map((opt) => (
        <option
          key={String(opt.value)}
          value={String(opt.value)}
          className="bg-white dark:bg-neutral-800 text-neutral-900 dark:text-neutral-50"
        >
          {opt.label}
        </option>
      ))}
    </select>
  )
}
