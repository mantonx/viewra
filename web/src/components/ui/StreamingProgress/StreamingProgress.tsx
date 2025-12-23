import { forwardRef } from 'react'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { X, CheckCircle, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Progress } from '@/components/ui/Progress'
import type { StreamingProgressProps } from './StreamingProgress.types'

export const StreamingProgress = forwardRef<HTMLDivElement, StreamingProgressProps>(
  (
    {
      status,
      percent,
      done,
      error,
      onCancel,
      variant = 'standalone',
      className,
    },
    ref
  ) => {
    const displayPercent = percent ?? 0
    const isIndeterminate = percent === undefined && !done && !error

    if (variant === 'inline') {
      // Compact inline variant for use in list items
      return (
        <div ref={ref} className={cn('flex items-center gap-2 min-w-[180px]', className)}>
          <div className="flex-1">
            <div className="flex items-center justify-between mb-1">
              <span className={cn('text-[10px]', text.tertiary)}>
                {status || 'Processing...'}
              </span>
              <span className={cn('text-[10px] font-medium', text.secondary)}>
                {displayPercent}%
              </span>
            </div>
            <Progress value={displayPercent} size="xs" />
          </div>
          {onCancel && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onCancel}
              className="text-neutral-400 hover:text-neutral-600 p-1 min-h-0"
            >
              <X className="w-3.5 h-3.5" />
            </Button>
          )}
        </div>
      )
    }

    // Standalone variant with status icon
    return (
      <div
        ref={ref}
        className={cn(
          'py-3 px-3 rounded-lg',
          'bg-neutral-50 dark:bg-neutral-900/50',
          error && 'border border-red-200 dark:border-red-900',
          className
        )}
      >
        <div className="flex items-center justify-between gap-3">
          <div className="flex-1 min-w-0">
            <p className={cn('text-sm', error ? 'text-red-500' : text.secondary)}>
              {error || status || 'Processing...'}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {done ? (
              <CheckCircle className="w-4 h-4 text-emerald-500" />
            ) : error ? (
              <AlertCircle className="w-4 h-4 text-red-500" />
            ) : (
              <>
                <Progress
                  value={displayPercent}
                  size="sm"
                  className="w-20"
                />
                <span className={cn('text-xs tabular-nums w-10', text.secondary)}>
                  {isIndeterminate ? '...' : `${displayPercent}%`}
                </span>
                {onCancel && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={onCancel}
                    className="text-neutral-400 hover:text-neutral-600 p-1 min-h-0"
                  >
                    <X className="w-3.5 h-3.5" />
                  </Button>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    )
  }
)

StreamingProgress.displayName = 'StreamingProgress'
