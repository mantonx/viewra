import { forwardRef } from 'react'
import { cn } from '@/lib/utils'

export interface ProgressProps {
  /** Progress value from 0-100 */
  value: number
  /** Optional class name for the container */
  className?: string
  /** Color variant */
  variant?: 'default' | 'success' | 'warning' | 'error'
  /** Size of the progress bar */
  size?: 'xs' | 'sm' | 'md' | 'lg'
  /** Optional label text displayed above the bar */
  label?: string
  /** Show percentage value */
  showPercentage?: boolean
  /** Position of percentage (end = after bar, inside label row) */
  percentagePosition?: 'label' | 'end'
}

const sizeClasses = {
  xs: 'h-1',
  sm: 'h-1.5',
  md: 'h-2',
  lg: 'h-3',
}

const trackClasses = {
  default: 'bg-neutral-200 dark:bg-neutral-800',
  success: 'bg-green-100 dark:bg-green-950',
  warning: 'bg-yellow-100 dark:bg-yellow-950',
  error: 'bg-red-100 dark:bg-red-950',
}

const fillClasses = {
  default: 'bg-primary-500',
  success: 'bg-green-500',
  warning: 'bg-yellow-500',
  error: 'bg-red-500',
}

export const Progress = forwardRef<HTMLDivElement, ProgressProps>(
  (
    {
      value,
      className,
      variant = 'default',
      size = 'md',
      label,
      showPercentage = false,
      percentagePosition = 'label',
    },
    ref
  ) => {
    const clampedValue = Math.min(Math.max(value, 0), 100)
    const hasHeader = label || (showPercentage && percentagePosition === 'label')

    return (
      <div ref={ref} className={cn('w-full', className)}>
        {hasHeader && (
          <div className="flex justify-between items-center mb-1.5">
            {label && (
              <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300">
                {label}
              </span>
            )}
            {showPercentage && percentagePosition === 'label' && (
              <span className="text-sm font-medium text-neutral-600 dark:text-neutral-400">
                {Math.round(clampedValue)}%
              </span>
            )}
          </div>
        )}
        <div
          className={cn(
            'w-full rounded-full overflow-hidden',
            sizeClasses[size],
            trackClasses[variant]
          )}
        >
          <div
            className={cn(
              'h-full rounded-full transition-all duration-300 ease-out',
              fillClasses[variant]
            )}
            style={{ width: `${clampedValue}%` }}
            role="progressbar"
            aria-valuenow={clampedValue}
            aria-valuemin={0}
            aria-valuemax={100}
          />
        </div>
        {showPercentage && percentagePosition === 'end' && (
          <div className="text-xs text-neutral-600 dark:text-neutral-400 mt-1 text-right">
            {Math.round(clampedValue)}%
          </div>
        )}
      </div>
    )
  }
)

Progress.displayName = 'Progress'
