import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { SourceBadge } from '../badges'
import type { ReadOnlySettingRowProps } from './DynamicSettingRow.types'

/**
 * Read-only setting row for displaying locked or detected values.
 * Used when a setting cannot be edited (e.g., locked by env var or auto-detected).
 *
 * @example
 * <ReadOnlySettingRow
 *   fieldKey="server.port"
 *   label="Server Port"
 *   value={8080}
 *   sourceBadge={{ source: 'env_var', envVar: 'PORT', locked: true }}
 * />
 */
export const ReadOnlySettingRow = ({
  fieldKey,
  label,
  description,
  value,
  sourceBadge,
  className,
}: ReadOnlySettingRowProps) => {
  const displayLabel = label || fieldKey

  // Format value for display
  const displayValue = (() => {
    if (typeof value === 'boolean') {
      return value ? 'Enabled' : 'Disabled'
    }
    if (value === null || value === undefined) {
      return '—'
    }
    return String(value)
  })()

  return (
    <div className={cn('flex items-start justify-between gap-4 py-3', className)}>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={cn('text-sm font-medium', text.primary)}>
            {displayLabel}
          </span>
          {sourceBadge && <SourceBadge {...sourceBadge} />}
        </div>
        {description && (
          <p className={cn('text-xs mt-1', text.tertiary)}>{description}</p>
        )}
      </div>
      <div
        className={cn(
          'px-3 py-1.5 rounded-md text-sm font-mono shrink-0',
          'bg-neutral-100 dark:bg-neutral-800',
          text.secondary
        )}
      >
        {displayValue}
      </div>
    </div>
  )
}
