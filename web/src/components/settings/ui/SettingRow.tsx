import { cn } from '@/lib/utils'
import { Toggle } from '@/components/ui/Toggle'
import { Select, type SelectOption } from '@/components/ui/Select'
import type { ReactNode } from 'react'

export interface SettingRowBaseProps {
  label: string
  description?: string
  disabled?: boolean
  className?: string
}

export interface SettingRowToggleProps extends SettingRowBaseProps {
  type: 'toggle'
  value: boolean
  onChange: (value: boolean) => void
}

export interface SettingRowSelectProps extends SettingRowBaseProps {
  type: 'select'
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
}

export interface SettingRowCustomProps extends SettingRowBaseProps {
  type: 'custom'
  children: ReactNode
}

export interface SettingRowReadonlyProps extends SettingRowBaseProps {
  type: 'readonly'
  value: string
}

export type SettingRowProps =
  | SettingRowToggleProps
  | SettingRowSelectProps
  | SettingRowCustomProps
  | SettingRowReadonlyProps

/**
 * SettingRow - A single setting item with label, description, and control
 *
 * Supports four types:
 * - toggle: Boolean switch
 * - select: Dropdown selection
 * - readonly: Display-only value (grayed out)
 * - custom: Any custom control passed as children
 */
export const SettingRow = (props: SettingRowProps) => {
  const { label, description, disabled = false, className, type } = props

  // Toggle type - horizontal layout with toggle on right
  if (type === 'toggle') {
    const { value, onChange } = props
    return (
      <div
        className={cn(
          'flex items-center justify-between p-4 rounded-xl transition-all duration-150',
          'bg-white/80 dark:bg-white/5',
          'border border-neutral-200/50 dark:border-white/10',
          disabled && 'opacity-50',
          className
        )}
      >
        <div className="flex-1 min-w-0 pr-4">
          <span className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
            {label}
          </span>
          {description && (
            <p className="text-xs mt-0.5 text-neutral-500 dark:text-neutral-400">
              {description}
            </p>
          )}
        </div>
        <Toggle
          enabled={value}
          onChange={onChange}
          label={label}
          disabled={disabled}
        />
      </div>
    )
  }

  // Select type - vertical layout with dropdown below label
  if (type === 'select') {
    const { value, onChange, options } = props
    return (
      <div className={cn('space-y-2', className)}>
        <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300">
          {label}
        </label>
        <Select
          options={options}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className="w-full"
        />
        {description && (
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            {description}
          </p>
        )}
      </div>
    )
  }

  // Readonly type - display value without editing
  if (type === 'readonly') {
    const { value } = props
    return (
      <div className={cn('space-y-2', className)}>
        <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300">
          {label}
        </label>
        <div
          className={cn(
            'w-full px-3 py-2 rounded-lg',
            'bg-neutral-100 dark:bg-white/5',
            'text-neutral-500 dark:text-neutral-400',
            'border border-neutral-200/50 dark:border-white/10'
          )}
        >
          {value}
        </div>
        {description && (
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            {description}
          </p>
        )}
      </div>
    )
  }

  // Custom type - render children
  if (type === 'custom') {
    const { children } = props
    return (
      <div className={cn('space-y-2', className)}>
        <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300">
          {label}
        </label>
        {children}
        {description && (
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            {description}
          </p>
        )}
      </div>
    )
  }

  return null
}
