import { cn } from '@/lib/utils'
import type { ProgressBarProps } from './ProgressBar.types'

export const ProgressBar = ({
  progress,
  label,
  variant = 'default',
  size = 'md',
  showPercentage = false,
}: ProgressBarProps) => {
  const clampedProgress = Math.min(Math.max(progress, 0), 100)

  const variants = {
    default: 'bg-blue-600',
    success: 'bg-green-600',
    warning: 'bg-yellow-500',
    error: 'bg-red-600',
  }

  const sizes = {
    sm: 'h-1.5',
    md: 'h-2',
  }

  const backgrounds = {
    default: 'bg-blue-100 dark:bg-blue-950',
    success: 'bg-green-100 dark:bg-green-950',
    warning: 'bg-yellow-100 dark:bg-yellow-950',
    error: 'bg-red-100 dark:bg-red-950',
  }

  return (
    <div className="w-full">
      {(label || showPercentage) && (
        <div className="flex justify-between items-center mb-1.5">
          {label && <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300">{label}</span>}
          {showPercentage && (
            <span className="text-sm font-medium text-neutral-600 dark:text-neutral-400">{clampedProgress.toFixed(1)}%</span>
          )}
        </div>
      )}
      <div className={cn('w-full rounded-full overflow-hidden', sizes[size], backgrounds[variant])}>
        <div
          className={cn('h-full transition-all duration-300 ease-out rounded-full', variants[variant])}
          style={{ width: `${clampedProgress}%` }}
        />
      </div>
    </div>
  )
}

export type { ProgressBarProps } from './ProgressBar.types'
