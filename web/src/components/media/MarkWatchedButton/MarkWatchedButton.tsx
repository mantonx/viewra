import { useState } from 'react'
import { Check, Eye, EyeOff, Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useMarkWatched, useMarkUnwatched } from '@/lib/hooks/useProgress'

interface MarkWatchedButtonProps {
  /** Media ID to mark as watched/unwatched */
  mediaId: number
  /** Whether the item is currently watched */
  isWatched: boolean
  /** Callback when watched state changes */
  onStateChange?: (isWatched: boolean) => void
  /** Visual variant */
  variant?: 'icon' | 'button' | 'compact'
  /** Additional CSS classes */
  className?: string
}

/**
 * MarkWatchedButton - Toggle button to mark media as watched/unwatched
 *
 * Provides a quick way to mark items as watched from media cards and rows.
 * Shows loading state during the API call and optimistically updates UI.
 */
export const MarkWatchedButton = ({
  mediaId,
  isWatched,
  onStateChange,
  variant = 'icon',
  className,
}: MarkWatchedButtonProps) => {
  const [optimisticWatched, setOptimisticWatched] = useState(isWatched)
  const markWatched = useMarkWatched()
  const markUnwatched = useMarkUnwatched()

  const isLoading = markWatched.isPending || markUnwatched.isPending

  const handleClick = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    if (isLoading) return

    const newState = !optimisticWatched
    setOptimisticWatched(newState)

    try {
      if (newState) {
        await markWatched.mutateAsync({ media_id: mediaId })
      } else {
        await markUnwatched.mutateAsync({ media_id: mediaId })
      }
      onStateChange?.(newState)
    } catch {
      // Revert optimistic update on error
      setOptimisticWatched(!newState)
    }
  }

  if (variant === 'icon') {
    return (
      <button
        onClick={handleClick}
        disabled={isLoading}
        className={cn(
          'p-1.5 rounded-full transition-all duration-200',
          'focus:outline-none focus:ring-2 focus:ring-primary-500/50',
          optimisticWatched
            ? 'bg-green-500 text-white hover:bg-green-600'
            : 'bg-neutral-900/75 text-white hover:bg-neutral-900/90 backdrop-blur-sm',
          isLoading && 'opacity-50 cursor-not-allowed',
          className
        )}
        title={optimisticWatched ? 'Mark as unwatched' : 'Mark as watched'}
        aria-label={optimisticWatched ? 'Mark as unwatched' : 'Mark as watched'}
      >
        {isLoading ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : optimisticWatched ? (
          <Check className="w-4 h-4" />
        ) : (
          <Eye className="w-4 h-4" />
        )}
      </button>
    )
  }

  if (variant === 'compact') {
    return (
      <button
        onClick={handleClick}
        disabled={isLoading}
        className={cn(
          'flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium',
          'transition-all duration-200',
          'focus:outline-none focus:ring-2 focus:ring-primary-500/50',
          optimisticWatched
            ? 'bg-green-500/20 text-green-600 dark:text-green-400 hover:bg-green-500/30'
            : 'bg-neutral-200 dark:bg-neutral-700 text-neutral-600 dark:text-neutral-300 hover:bg-neutral-300 dark:hover:bg-neutral-600',
          isLoading && 'opacity-50 cursor-not-allowed',
          className
        )}
        title={optimisticWatched ? 'Mark as unwatched' : 'Mark as watched'}
      >
        {isLoading ? (
          <Loader2 className="w-3 h-3 animate-spin" />
        ) : optimisticWatched ? (
          <>
            <Check className="w-3 h-3" />
            <span>Watched</span>
          </>
        ) : (
          <>
            <Eye className="w-3 h-3" />
            <span>Watch</span>
          </>
        )}
      </button>
    )
  }

  // Full button variant
  return (
    <button
      onClick={handleClick}
      disabled={isLoading}
      className={cn(
        'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium',
        'transition-all duration-200',
        'focus:outline-none focus:ring-2 focus:ring-primary-500/50',
        optimisticWatched
          ? 'bg-green-500 text-white hover:bg-green-600'
          : 'bg-neutral-200 dark:bg-neutral-700 text-neutral-900 dark:text-white hover:bg-neutral-300 dark:hover:bg-neutral-600',
        isLoading && 'opacity-50 cursor-not-allowed',
        className
      )}
    >
      {isLoading ? (
        <Loader2 className="w-4 h-4 animate-spin" />
      ) : optimisticWatched ? (
        <EyeOff className="w-4 h-4" />
      ) : (
        <Eye className="w-4 h-4" />
      )}
      <span>{optimisticWatched ? 'Mark Unwatched' : 'Mark Watched'}</span>
    </button>
  )
}
