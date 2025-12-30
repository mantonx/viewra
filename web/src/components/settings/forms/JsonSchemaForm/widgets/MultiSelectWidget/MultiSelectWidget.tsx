import { cn } from '@/lib/utils'
import { Check } from 'lucide-react'
import type { MultiSelectWidgetProps } from './MultiSelectWidget.types'

export const MultiSelectWidget = ({
  id,
  value = [],
  onChange,
  options,
  disabled,
  readonly,
}: MultiSelectWidgetProps) => {
  const enumOptions = options.enumOptions ?? []

  const toggleOption = (optValue: string) => {
    if (disabled || readonly) {
      return
    }

    const newValue = value.includes(optValue)
      ? value.filter((v) => v !== optValue)
      : [...value, optValue]
    onChange(newValue)
  }

  return (
    <div id={id} className="flex flex-wrap gap-2">
      {enumOptions.map((opt) => {
        const optValue = String(opt.value)
        const isSelected = value.includes(optValue)

        return (
          <button
            key={optValue}
            type="button"
            onClick={() => toggleOption(optValue)}
            disabled={disabled || readonly}
            className={cn(
              'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-all duration-150',
              'border',
              // Unselected state
              !isSelected && [
                'bg-white dark:bg-white/5',
                'text-neutral-600 dark:text-neutral-400',
                'border-neutral-200/50 dark:border-white/10',
                'hover:border-neutral-300 dark:hover:border-white/20',
                'hover:text-neutral-900 dark:hover:text-neutral-200',
              ],
              // Selected state
              isSelected && [
                'bg-primary-500/10 dark:bg-primary-500/20',
                'text-primary-700 dark:text-primary-300',
                'border-primary-500/30 dark:border-primary-500/40',
              ],
              // Disabled
              'disabled:opacity-50 disabled:cursor-not-allowed'
            )}
          >
            {isSelected && <Check className="w-3.5 h-3.5" />}
            {opt.label}
          </button>
        )
      })}
    </div>
  )
}
