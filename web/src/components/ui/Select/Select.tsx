import { cn } from '@/lib/utils'
import type { SelectHTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface SelectOption {
  value: string
  label: string
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
  helperText?: string
  options: SelectOption[]
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, label, error, helperText, options, id, ...props }, ref) => {
    const selectId = id || label?.toLowerCase().replace(/\s+/g, '-')

    return (
      <div className="w-full">
        {label && (
          <label htmlFor={selectId} className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1">
            {label}
            {props.required && <span className="text-red-500 ml-1">*</span>}
          </label>
        )}
        <select
          id={selectId}
          ref={ref}
          className={cn(
            'w-full px-3 py-2 border rounded-md transition-colors',
            'bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50',
            'focus:ring-2 focus:ring-blue-500 focus:border-blue-500',
            'disabled:bg-neutral-50 dark:disabled:bg-neutral-950 disabled:text-neutral-500 dark:disabled:text-neutral-500 disabled:cursor-not-allowed',
            error ? 'border-red-300 dark:border-red-700 focus:border-red-500 focus:ring-red-500' : 'border-neutral-300 dark:border-neutral-700',
            className
          )}
          {...props}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        {error && <p className="mt-1 text-sm text-red-600 dark:text-red-400">{error}</p>}
        {helperText && !error && <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-500">{helperText}</p>}
      </div>
    )
  }
)

Select.displayName = 'Select'
