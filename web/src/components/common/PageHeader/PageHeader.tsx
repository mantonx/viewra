import type { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: ReactNode
  className?: string
}

/**
 * PageHeader component for consistent page titles and actions
 *
 * @example
 * ```tsx
 * <PageHeader
 *   title="Libraries"
 *   description="Manage your media libraries"
 *   actions={<Button onClick={handleAdd}>+ Add Library</Button>}
 * />
 * ```
 */
const PageHeader = ({ title, description, actions, className = '' }: PageHeaderProps) => {
  return (
    <div className={`mb-6 ${className}`}>
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold mb-2 text-neutral-900 dark:text-neutral-50">{title}</h1>
          {description && <p className="text-neutral-600 dark:text-neutral-400">{description}</p>}
        </div>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
    </div>
  )
}

export { PageHeader }
export type { PageHeaderProps }
