import { cn } from '@/lib/utils'
import type { InputHTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, helperText, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-')

    return (
      <div className="w-full">
        {label && (
          <label htmlFor={inputId} className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1">
            {label}
            {props.required && <span className="text-red-500 dark:text-red-400 ml-1">*</span>}
          </label>
        )}
        <input
          id={inputId}
          ref={ref}
          className={cn(
            'w-full px-3 py-2 border rounded-md transition-colors',
            'bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50',
            'focus:ring-2 focus:ring-blue-500 focus:border-blue-500',
            'disabled:bg-neutral-50 dark:disabled:bg-neutral-800 disabled:text-neutral-500 dark:disabled:text-neutral-600 disabled:cursor-not-allowed',
            error ? 'border-red-300 dark:border-red-800 focus:border-red-500 focus:ring-red-500' : 'border-neutral-300 dark:border-neutral-700',
            className
          )}
          {...props}
        />
        {error && <p className="mt-1 text-sm text-red-600 dark:text-red-400">{error}</p>}
        {helperText && !error && <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">{helperText}</p>}
      </div>
    )
  }
)

Input.displayName = 'Input'
