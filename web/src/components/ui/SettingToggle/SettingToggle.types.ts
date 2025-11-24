import type { ReactNode } from 'react'

export interface SettingToggleProps {
  /**
   * Whether the toggle is enabled
   */
  enabled: boolean

  /**
   * Callback when toggle state changes
   */
  onChange: (enabled: boolean) => void

  /**
   * Label for the setting
   */
  label: string

  /**
   * Description text explaining what the setting does
   */
  description: string

  /**
   * Accessibility label for the toggle
   */
  ariaLabel: string

  /**
   * Optional content to show on the right side when enabled
   */
  previewContent?: ReactNode

  /**
   * Whether the setting is disabled
   */
  disabled?: boolean
}
