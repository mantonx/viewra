export interface CheckboxWidgetProps {
  /** Input element ID */
  id: string
  /** Current value */
  value?: boolean
  /** Change handler */
  onChange: (value: boolean) => void
  /** Label text */
  label?: string
  /** Whether the checkbox is disabled */
  disabled?: boolean
  /** Whether the checkbox is read-only */
  readonly?: boolean
  /** Schema definition with description */
  schema?: { description?: string }
}
