import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

export interface SettingSectionProps {
  /** Section title (e.g., "Display Settings") */
  title: string
  /** Section description */
  description: string
  /** Section content */
  children: ReactNode
  /** Optional restart notice for the entire section */
  restartRequired?: boolean
  /** Additional CSS classes */
  className?: string
}

/**
 * SettingSection - Page-level header and container for a settings category
 *
 * Matches the Figma mockup's layout with:
 * - Large title and description at top
 * - Optional restart notice (shown once per section, not per field)
 * - Content area for SettingGroup components
 */
export const SettingSection = ({
  title,
  description,
  children,
  restartRequired = false,
  className,
}: SettingSectionProps) => {
  return (
    <div className={cn('space-y-6', className)}>
      {/* Section header */}
      <div>
        <h2 className="text-xl font-semibold text-neutral-900 dark:text-neutral-50">
          {title}
        </h2>
        <p className="text-sm mt-1 text-neutral-500 dark:text-neutral-400">
          {description}
        </p>
      </div>

      {/* Restart notice - shown once per section if any setting requires restart */}
      {restartRequired && (
        <div
          className={cn(
            'flex items-center gap-2 px-4 py-3 rounded-lg',
            'bg-amber-50 dark:bg-amber-500/10',
            'border border-amber-200 dark:border-amber-500/20',
            'text-amber-700 dark:text-amber-400'
          )}
        >
          <svg
            className="w-4 h-4 flex-shrink-0"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <span className="text-sm">
            Some changes require a server restart to take effect
          </span>
        </div>
      )}

      {/* Section content */}
      {children}
    </div>
  )
}
