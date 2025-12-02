import { cn } from '@/lib/utils'
import { Toggle } from '../Toggle'
import type { SettingToggleProps } from './SettingToggle.types'

export const SettingToggle = ({
  enabled,
  onChange,
  label,
  description,
  ariaLabel,
  previewContent,
  disabled = false,
}: SettingToggleProps) => {
  return (
    <div
      className={cn(
        'flex items-center justify-between p-4 rounded-xl transition-all duration-150',
        'bg-white/80 dark:bg-white/5',
        'border border-neutral-200/50 dark:border-white/10',
        'backdrop-blur-sm',
        'hover:bg-white dark:hover:bg-white/[0.07]'
      )}
    >
      <div className="flex-1">
        <div className="flex items-center gap-3">
          <Toggle
            enabled={enabled}
            onChange={onChange}
            label={ariaLabel}
            disabled={disabled}
          />
          <div>
            <span className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
              {label}
            </span>
            <p className="text-xs mt-0.5 text-neutral-500 dark:text-neutral-500">
              {description}
            </p>
          </div>
        </div>
      </div>
      {previewContent && enabled && (
        <div className="ml-4">
          {previewContent}
        </div>
      )}
    </div>
  )
}
