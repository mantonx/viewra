import type { ReactNode } from 'react'

export interface EmptyStateProps {
  /**
   * Icon or emoji to display (e.g., "🎬" or a component)
   */
  icon: ReactNode

  /**
   * Title text for the empty state
   */
  title: string

  /**
   * Description text explaining the empty state
   */
  description: string

  /**
   * Optional action button or component to display
   */
  action?: ReactNode

  /**
   * Additional CSS classes
   */
  className?: string
}
