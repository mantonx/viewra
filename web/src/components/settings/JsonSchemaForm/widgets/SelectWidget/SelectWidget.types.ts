import type { EnumOptionsType } from '@rjsf/utils'

export interface SelectWidgetProps {
  /** Select element ID */
  id: string
  /** Current value */
  value?: string
  /** Change handler */
  onChange: (value: string) => void
  /** RJSF options containing enum options */
  options: { enumOptions?: EnumOptionsType[] }
  /** Whether the select is disabled */
  disabled?: boolean
  /** Whether the select is read-only */
  readonly?: boolean
  /** Placeholder text */
  placeholder?: string
}
