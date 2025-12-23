export interface TextWidgetProps {
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
  /** Whether to auto-focus the input */
  autofocus?: boolean
  /** Widget options from RJSF */
  options?: { inputType?: string }
  /** Schema definition */
  schema?: { format?: string }
}
