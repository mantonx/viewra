import { cn } from '@/lib/utils'

const Skeleton = ({ className = '' }: { className?: string }) => {
  return (
    <div
      className={cn(
        'rounded bg-neutral-200 dark:bg-neutral-800',
        'skeleton-shimmer',
        className
      )}
      aria-hidden="true"
    />
  )
}

export { Skeleton }
