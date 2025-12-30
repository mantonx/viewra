import type { EnumOptionsType } from '@rjsf/utils'

export interface MultiSelectWidgetProps {
  /** Widget element ID */
  id: string
  /** Current selected values */
  value?: string[]
  /** Change handler */
  onChange: (value: string[]) => void
  /** RJSF options containing enum options */
  options: { enumOptions?: EnumOptionsType[] }
  /** Whether the widget is disabled */
  disabled?: boolean
  /** Whether the widget is read-only */
  readonly?: boolean
  /** Placeholder text */
  placeholder?: string
}
