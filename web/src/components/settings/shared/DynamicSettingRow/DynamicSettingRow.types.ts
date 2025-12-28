import type { ReactNode } from 'react'
import type { useSettingsForm } from '@/lib/hooks'
import type { SourceBadgeProps } from '../badges'

/**
 * Base props for setting rows
 */
export type BaseSettingRowProps = {
  /** Setting key/name */
  fieldKey: string
  /** Display label */
  label?: string
  /** Description text */
  description?: string
  /** Whether the setting has been changed from original */
  isChanged?: boolean
  /** Whether the setting is disabled */
  disabled?: boolean
  /** Additional class name */
  className?: string
}

/**
 * Props for DynamicSettingRow component
 */
export type DynamicSettingRowProps = BaseSettingRowProps & {
  /** Setting type determines which input to render */
  type?: 'string' | 'int' | 'bool' | string
  /** Options for select fields */
  options?: { value: string; label: string }[]
  /** TanStack Form instance from useSettingsForm */
  form: ReturnType<typeof useSettingsForm>['form']
  /** Source badge configuration */
  sourceBadge?: SourceBadgeProps
  /** Whether to show restart badge */
  showRestartBadge?: boolean
  /** Custom content to render after the input */
  suffix?: ReactNode
}

/**
 * Props for ReadOnlySettingRow component
 */
export type ReadOnlySettingRowProps = BaseSettingRowProps & {
  /** Current value to display */
  value: unknown
  /** Source badge configuration */
  sourceBadge?: SourceBadgeProps
}
