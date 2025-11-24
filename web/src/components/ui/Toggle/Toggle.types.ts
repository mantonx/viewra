export interface ToggleProps {
  /**
   * Whether the toggle is enabled/on
   */
  enabled: boolean

  /**
   * Callback when toggle state changes
   */
  onChange: (enabled: boolean) => void

  /**
   * Label for accessibility
   */
  label: string

  /**
   * Optional description (not rendered, just for context)
   */
  description?: string

  /**
   * Whether the toggle is disabled
   */
  disabled?: boolean

  /**
   * Additional CSS classes
   */
  className?: string
}
