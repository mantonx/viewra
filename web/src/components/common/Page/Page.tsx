import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

export interface SettingsPageProps {
  children: ReactNode
  className?: string
}

export interface SettingsPageHeaderProps {
  title: string
  description?: string
  actions?: ReactNode
  className?: string
}

export interface SettingsPageCardProps {
  title?: ReactNode
  description?: string
  children: ReactNode
  className?: string
}

export interface SettingsPageListProps {
  children: ReactNode
  className?: string
}

/**
 * SettingsPage - Scroll wrapper and container for management/settings pages
 *
 * Provides consistent layout with:
 * - Full-height scrollable container
 * - Padded content area with max-width (4xl)
 *
 * @example
 * ```tsx
 * <SettingsPage>
 *   <SettingsPage.Header title="Libraries" description="Manage your media" />
 *   <SettingsPage.Card title="Your Libraries">
 *     <SettingsPage.List>
 *       {items.map(item => <LibraryCard key={item.id} />)}
 *     </SettingsPage.List>
 *   </SettingsPage.Card>
 * </SettingsPage>
 * ```
 */
const SettingsPageRoot = ({ children, className }: SettingsPageProps) => {
  return (
    <div className="h-full overflow-auto">
      <div className={cn('p-6 max-w-4xl', className)}>{children}</div>
    </div>
  )
}

/**
 * SettingsPage.Header - Title, description, and optional actions
 */
const Header = ({
  title,
  description,
  actions,
  className,
}: SettingsPageHeaderProps) => {
  return (
    <div className={cn('flex items-start justify-between mb-6', className)}>
      <div>
        <h1 className="text-xl font-semibold text-neutral-900 dark:text-neutral-50">
          {title}
        </h1>
        {description && (
          <p className="text-sm mt-1 text-neutral-500 dark:text-neutral-400">
            {description}
          </p>
        )}
      </div>
      {actions && <div className="flex gap-3 ml-4">{actions}</div>}
    </div>
  )
}

/**
 * SettingsPage.Card - Glass card container with optional title
 */
const Card = ({
  title,
  description,
  children,
  className,
}: SettingsPageCardProps) => {
  return (
    <div
      className={cn(
        'rounded-xl',
        'bg-white/50 dark:bg-white/[0.02]',
        'border border-neutral-200/50 dark:border-white/10',
        className
      )}
    >
      {(title || description) && (
        <div className="p-5 pb-0">
          {title && (
            <h2 className="text-base font-semibold text-neutral-900 dark:text-neutral-50">
              {title}
            </h2>
          )}
          {description && (
            <p className="text-sm mt-1 text-neutral-500 dark:text-neutral-400">
              {description}
            </p>
          )}
        </div>
      )}
      <div className="p-5">{children}</div>
    </div>
  )
}

/**
 * SettingsPage.List - Bordered container for list items with dividers
 */
const List = ({ children, className }: SettingsPageListProps) => {
  return (
    <div
      className={cn(
        'divide-y divide-neutral-200/50 dark:divide-white/10',
        'rounded-lg overflow-hidden',
        'border border-neutral-200/50 dark:border-white/10',
        className
      )}
    >
      {children}
    </div>
  )
}

// Compound component pattern
export const SettingsPage = Object.assign(SettingsPageRoot, {
  Header,
  Card,
  List,
})
