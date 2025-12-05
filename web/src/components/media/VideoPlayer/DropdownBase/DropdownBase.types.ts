import type { ReactNode } from 'react'

export interface DropdownBaseProps {
  /** Content to display on the button */
  buttonContent: ReactNode
  /** Icon to show before the button content */
  icon?: ReactNode
  /** Minimum width for the button */
  minButtonWidth?: string
  /** Aria label for the button */
  ariaLabel: string
  /** Children rendered inside the panel */
  children: (props: { close: () => void }) => ReactNode
  /** Width class for the panel (e.g., 'w-56', 'w-80') */
  panelWidth?: string
}
