import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { CheckCircle, AlertCircle } from 'lucide-react'
import { Progress } from '@/components/ui'
import type { ProgressDisplayProps } from './ProgressDisplay.types'

export const ProgressDisplay = ({ progress, label }: ProgressDisplayProps) => {
  const percent = progress.percent ??
    (progress.completed && progress.total
      ? Math.round((progress.completed / progress.total) * 100)
      : 0)

  return (
    <div
      className={cn(
        'py-3 px-3 rounded-lg',
        'bg-neutral-50 dark:bg-neutral-900/50',
        progress.error && 'border border-red-200 dark:border-red-900'
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex-1 min-w-0">
          {label && (
            <span className={cn('text-sm font-medium', text.primary)}>
              {label}
            </span>
          )}
          <p className={cn('text-xs mt-0.5', progress.error ? 'text-red-500' : text.secondary)}>
            {progress.error || progress.status}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {progress.done ? (
            <CheckCircle className="w-4 h-4 text-emerald-500" />
          ) : progress.error ? (
            <AlertCircle className="w-4 h-4 text-red-500" />
          ) : (
            <>
              <Progress value={percent} size="sm" className="w-20" />
              <span className={cn('text-xs tabular-nums w-10', text.secondary)}>
                {percent}%
              </span>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

ProgressDisplay.displayName = 'ProgressDisplay'
