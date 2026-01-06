import { useMemo } from 'react'
import { cn } from '@/lib/utils'

interface NewBadgeProps {
  /** ISO date string when the item was created/added */
  createdAt: string
  /** Number of days to consider an item as "new" (default: 7) */
  daysThreshold?: number
  /** Additional CSS classes */
  className?: string
}

/**
 * NewBadge - "New" indicator for recently added items
 *
 * Shows a "New" pill badge on items that were added within the threshold period.
 * Typically used on recently added movies/shows in media rows.
 */
export const NewBadge = ({ createdAt, daysThreshold = 7, className }: NewBadgeProps) => {
  const isNew = useMemo(() => {
    if (!createdAt) return false

    const created = new Date(createdAt)
    if (isNaN(created.getTime())) return false

    const now = new Date()
    const diffMs = now.getTime() - created.getTime()
    const diffDays = diffMs / (1000 * 60 * 60 * 24)

    return diffDays <= daysThreshold
  }, [createdAt, daysThreshold])

  if (!isNew) {
    return null
  }

  return (
    <span
      className={cn(
        'px-2 py-0.5 text-xs font-semibold rounded-full',
        'bg-primary-500 text-white',
        'shadow-sm',
        className
      )}
    >
      New
    </span>
  )
}
