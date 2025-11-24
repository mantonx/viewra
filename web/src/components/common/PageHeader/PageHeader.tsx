import type { ReactNode } from 'react'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'

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
          <h1 className={cn('text-3xl font-bold mb-2', text.primary)}>{title}</h1>
          {description && <p className={cn(text.secondary)}>{description}</p>}
        </div>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
    </div>
  )
}

export { PageHeader }
export type { PageHeaderProps }
