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
      info: 'bg-blue-50 border-blue-200 text-blue-800',
      success: 'bg-green-50 border-green-200 text-green-800',
      warning: 'bg-yellow-50 border-yellow-200 text-yellow-800',
      error: 'bg-red-50 border-red-200 text-red-800',
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
        className={cn('border rounded-lg p-4', variants[variant], className)}
        role="alert"
        {...props}
      >
        <div className="flex gap-3">
          <span className="text-xl flex-shrink-0">{icons[variant]}</span>
          <div className="flex-1">
            {title && <h3 className="font-semibold mb-1">{title}</h3>}
            <div className="text-sm">{children}</div>
          </div>
        </div>
      </div>
    )
  }
)

Alert.displayName = 'Alert'
