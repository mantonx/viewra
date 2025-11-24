import { cn } from '@/lib/utils'
import { bg, text, border } from '@/styles/semantic'
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
        'flex items-center justify-between p-4 rounded-lg transition-colors',
        bg.elevated,
        border.primary,
        'border'
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
            <span className={cn('text-sm font-medium', text.primary)}>
              {label}
            </span>
            <p className={cn('text-xs mt-0.5', text.tertiary)}>
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
