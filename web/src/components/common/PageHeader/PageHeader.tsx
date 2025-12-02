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
    <div className={`mb-8 ${className}`}>
      <div className="flex justify-between items-start">
        <div>
          <h1 className={cn('text-3xl font-semibold mb-2 font-display tracking-tight leading-tight', text.primary)}>{title}</h1>
          {description && <p className={cn('text-base leading-relaxed', text.secondary)}>{description}</p>}
        </div>
        {actions && <div className="flex gap-3">{actions}</div>}
      </div>
    </div>
  )
}

export { PageHeader }
export type { PageHeaderProps }
