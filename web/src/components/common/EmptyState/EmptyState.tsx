import { isValidElement, type ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'

interface EmptyStateProps {
  /** Lucide icon component (preferred) or ReactNode for legacy emoji support */
  icon?: LucideIcon | ReactNode
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

/**
 * Check if a value is a Lucide icon component.
 * Lucide icons are ForwardRefExoticComponents with a $$typeof symbol and render function.
 */
const isLucideIcon = (icon: unknown): icon is LucideIcon => {
  return (
    typeof icon === 'object' &&
    icon !== null &&
    '$$typeof' in icon &&
    'render' in icon
  )
}

/**
 * EmptyState component for displaying empty or no-data states
 *
 * @example
 * ```tsx
 * import { Tv } from 'lucide-react'
 *
 * <EmptyState
 *   icon={Tv}
 *   title="No TV shows found"
 *   description="Add a library with TV shows to see them here"
 *   action={<Button onClick={handleAdd}>Add Library</Button>}
 * />
 * ```
 */
const EmptyState = ({ icon, title, description, action, className = '' }: EmptyStateProps) => {
  const renderIcon = () => {
    if (!icon) return null

    // Check if it's a Lucide icon component
    if (isLucideIcon(icon)) {
      const Icon = icon
      return (
        <div className="flex justify-center mb-4">
          <div className="w-20 h-20 rounded-full bg-neutral-100 dark:bg-neutral-800 flex items-center justify-center">
            <Icon className="w-10 h-10 text-neutral-400 dark:text-neutral-500" />
          </div>
        </div>
      )
    }

    // Check if it's a React element (already rendered)
    if (isValidElement(icon)) {
      return <div className="flex justify-center mb-4">{icon}</div>
    }

    // Fallback for strings (emojis) or other primitives
    return <div className="flex justify-center mb-4 text-6xl">{icon}</div>
  }

  return (
    <div className={cn('text-center py-12', className)}>
      {renderIcon()}
      <h3 className={cn('text-lg font-semibold mb-2', text.primary)}>{title}</h3>
      {description && <p className={cn('mb-4 max-w-md mx-auto', text.secondary)}>{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export { EmptyState }
export type { EmptyStateProps }
