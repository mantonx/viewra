/**
 * Types for EmptyState component.
 */

import type { ReactNode } from 'react'

export interface EmptyStateProps {
  /** Icon to display */
  icon?: ReactNode
  /** Title text */
  title: string
  /** Description text */
  description?: string
  /** Action button content */
  action?: ReactNode
  /** Variant style */
  variant?: 'default' | 'dashed'
  /** Size variant */
  size?: 'sm' | 'md' | 'lg'
  /** Additional class names */
  className?: string
}
