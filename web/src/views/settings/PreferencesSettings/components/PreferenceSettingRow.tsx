import { SourceBadge } from '@/components/settings'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { PreferenceSettingRowProps } from '../PreferencesSettings.types'

/**
 * A single row for a preference setting with label, description, and form control.
 */
export const PreferenceSettingRow = ({
  definition,
  isChanged,
  isDefault,
  children,
}: PreferenceSettingRowProps) => {
  return (
    <div
      className={cn(
        'py-4 border-l-2 pl-4 -ml-4 transition-colors',
        isChanged ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/20' : 'border-transparent'
      )}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <label className={cn('font-medium', text.primary)}>{definition.label}</label>
            <SourceBadge source={isDefault && !isChanged ? 'default' : 'database'} />
          </div>
          {definition.description && (
            <p className={cn('text-sm mt-0.5', text.secondary)}>{definition.description}</p>
          )}
        </div>
        <div className="shrink-0">{children}</div>
      </div>
    </div>
  )
}
