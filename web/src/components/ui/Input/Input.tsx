import { cn } from '@/lib/utils'
import type { InputHTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
  variant?: 'default' | 'glass'
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, helperText, id, variant = 'default', ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-')

    const variants = {
      default: [
        'bg-white dark:bg-white/5',
        'border-neutral-200 dark:border-white/10',
        'text-neutral-900 dark:text-neutral-50',
        'placeholder:text-neutral-400 dark:placeholder:text-neutral-500',
        'hover:border-neutral-300 dark:hover:border-white/20',
      ],
      glass: [
        'bg-white/10 dark:bg-white/5',
        'border-white/20 dark:border-white/10',
        'text-neutral-900 dark:text-white text-lg',
        'placeholder:text-neutral-400 dark:placeholder:text-neutral-500 placeholder:text-lg',
        'backdrop-blur-sm',
        'py-3.5 px-4',
        'hover:border-white/30 dark:hover:border-white/20',
      ],
    }

    const labelVariants = {
      default: 'text-neutral-700 dark:text-neutral-300 text-sm',
      glass: 'text-neutral-700 dark:text-neutral-200 text-sm tracking-wide uppercase',
    }

    const helperVariants = {
      default: 'text-neutral-500 dark:text-neutral-400',
      glass: 'text-neutral-500 dark:text-neutral-400',
    }

    return (
      <div className="w-full">
        {label && (
          <label htmlFor={inputId} className={cn('block font-medium mb-2', labelVariants[variant])}>
            {label}
            {props.required && <span className="text-red-500 dark:text-red-400 ml-1">*</span>}
          </label>
        )}
        <input
          id={inputId}
          ref={ref}
          className={cn(
            'w-full px-4 py-2.5 border rounded-lg transition-all duration-200 ease-out outline-none',
            'focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500 dark:focus:ring-primary-500/20',
            'disabled:opacity-50 disabled:cursor-not-allowed',
            error ? 'border-error-400 dark:border-error-600 focus:border-error-500 focus:ring-error-500/30' : variants[variant],
            className
          )}
          {...props}
        />
        {error && <p className="mt-1.5 text-sm text-red-600 dark:text-red-400">{error}</p>}
        {helperText && !error && <p className={cn('mt-1.5 text-sm', helperVariants[variant])}>{helperText}</p>}
      </div>
    )
  }
)

Input.displayName = 'Input'
