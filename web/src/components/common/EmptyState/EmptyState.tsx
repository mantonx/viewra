import type { ReactNode } from 'react'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

/**
 * EmptyState component for displaying empty or no-data states
 *
 * @example
 * ```tsx
 * <EmptyState
 *   icon="📚"
 *   title="No libraries yet"
 *   description="Add a library to get started"
 *   action={<Button onClick={handleAdd}>Add Library</Button>}
 * />
 * ```
 */
const EmptyState = ({ icon, title, description, action, className = '' }: EmptyStateProps) => {
  return (
    <div className={`text-center py-12 ${className}`}>
      {icon && <div className="text-6xl mb-4">{icon}</div>}
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
      {description && <p className="text-gray-600 mb-4">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export { EmptyState }
export type { EmptyStateProps }
