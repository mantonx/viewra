/**
 * Types for Badge component.
 */

export type BadgeColor =
  | 'blue'
  | 'green'
  | 'yellow'
  | 'red'
  | 'purple'
  | 'gray'
  | 'emerald'
  | 'primary'

export type BadgeSize = 'sm' | 'md'

export interface BadgeProps {
  /** Badge content */
  children: React.ReactNode
  /** Color variant */
  color?: BadgeColor
  /** Size variant */
  size?: BadgeSize
  /** Additional class names */
  className?: string
}
