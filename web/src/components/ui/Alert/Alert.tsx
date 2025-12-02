import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface AlertProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'info' | 'success' | 'warning' | 'error'
  title?: string
}

export const Alert = forwardRef<HTMLDivElement, AlertProps>(
  ({ className, variant = 'info', title, children, ...props }, ref) => {
    const variants = {
      info: cn(
        'bg-blue-50/80 border-blue-200/50 text-blue-800',
        'dark:bg-blue-500/10 dark:border-blue-500/20 dark:text-blue-300'
      ),
      success: cn(
        'bg-green-50/80 border-green-200/50 text-green-800',
        'dark:bg-green-500/10 dark:border-green-500/20 dark:text-green-300'
      ),
      warning: cn(
        'bg-amber-50/80 border-amber-200/50 text-amber-800',
        'dark:bg-amber-500/10 dark:border-amber-500/20 dark:text-amber-300'
      ),
      error: cn(
        'bg-red-50/80 border-red-200/50 text-red-800',
        'dark:bg-red-500/10 dark:border-red-500/20 dark:text-red-300'
      ),
    }

    const icons = {
      info: 'ℹ',
      success: '✓',
      warning: '⚠',
      error: '✕',
    }

    return (
      <div
        ref={ref}
        className={cn(
          'rounded-xl p-4 border backdrop-blur-sm',
          variants[variant],
          className
        )}
        role="alert"
        {...props}
      >
        <div className="flex gap-3">
          <span className="text-lg shrink-0 opacity-80">{icons[variant]}</span>
          <div className="flex-1">
            {title && <h3 className="font-semibold mb-1">{title}</h3>}
            <div className="text-sm opacity-90">{children}</div>
          </div>
        </div>
      </div>
    )
  }
)

Alert.displayName = 'Alert'
