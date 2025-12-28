import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

export interface SettingGroupProps {
  /** Optional title for the group (renders as h3) */
  title?: string
  /** Optional description below the title */
  description?: string
  /** Child settings rows */
  children: ReactNode
  /** Additional CSS classes */
  className?: string
  /** Info banner content (like "About AI Features" in mockup) */
  infoBanner?: {
    icon?: ReactNode
    title: string
    description: string
  }
}

/**
 * SettingGroup - A card container for related settings
 *
 * Provides visual grouping with optional header and info banner.
 * Matches the Figma mockup's bordered card with rounded corners.
 */
export const SettingGroup = ({
  title,
  description,
  children,
  className,
  infoBanner,
}: SettingGroupProps) => {
  return (
    <div
      className={cn(
        'rounded-xl',
        'bg-white/50 dark:bg-white/[0.02]',
        'border border-neutral-200/50 dark:border-white/10',
        className
      )}
    >
      {/* Optional section header inside the card */}
      {(title || description) && (
        <div className="p-5 pb-0">
          {title && (
            <h3 className="text-base font-semibold text-neutral-900 dark:text-neutral-50">
              {title}
            </h3>
          )}
          {description && (
            <p className="text-sm mt-1 text-neutral-500 dark:text-neutral-400">
              {description}
            </p>
          )}
        </div>
      )}

      {/* Info banner (like "About AI Features") */}
      {infoBanner && (
        <div
          className={cn(
            'm-5 p-4 rounded-lg',
            'bg-primary-50/50 dark:bg-primary-500/10',
            'border border-primary-200/50 dark:border-primary-500/20'
          )}
        >
          <div className="flex gap-3">
            {infoBanner.icon && (
              <div className="flex-shrink-0 text-primary-500">
                {infoBanner.icon}
              </div>
            )}
            <div>
              <h4 className="text-sm font-medium text-primary-700 dark:text-primary-300">
                {infoBanner.title}
              </h4>
              <p className="text-xs mt-1 text-primary-600/80 dark:text-primary-400/80">
                {infoBanner.description}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Settings content */}
      <div className="p-5 space-y-4">{children}</div>
    </div>
  )
}
