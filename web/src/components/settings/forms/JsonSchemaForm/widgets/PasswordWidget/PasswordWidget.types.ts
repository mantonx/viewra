export interface PasswordWidgetProps {
  /** Input element ID */
  id: string
  /** Current value */
  value?: string
  /** Change handler */
  onChange: (value: string) => void
  /** Placeholder text */
  placeholder?: string
  /** Whether the input is disabled */
  disabled?: boolean
  /** Whether the input is read-only */
  readonly?: boolean
}
