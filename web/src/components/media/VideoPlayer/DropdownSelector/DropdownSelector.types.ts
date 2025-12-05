import type { ReactNode } from 'react'

export interface DropdownOption<T> {
  value: T
  label: string
  sublabel?: string
  icon?: ReactNode
  isSelected?: boolean
}

export interface DropdownSelectorProps<T> {
  /** Current display text shown on the button */
  displayText: string
  /** Icon to show before the display text */
  icon?: ReactNode
  /** Panel header title */
  title: string
  /** Optional subtitle/description in the header */
  subtitle?: string
  /** Options to display */
  options: DropdownOption<T>[]
  /** Called when an option is selected */
  onSelect: (value: T) => void
  /** Minimum width for the button */
  minButtonWidth?: string
  /** Width for the dropdown panel */
  panelWidth?: string
  /** Aria label for the button */
  ariaLabel: string
  /** Optional footer content */
  footer?: ReactNode
}
