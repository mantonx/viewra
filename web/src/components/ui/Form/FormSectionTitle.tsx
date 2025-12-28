import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

type FormSectionTitleProps = {
  /** Section title text */
  children: ReactNode
  /** Additional class names */
  className?: string
}

/**
 * Section heading for grouping form fields.
 *
 * @example
 * <FormSectionTitle>Advanced Settings</FormSectionTitle>
 */
export const FormSectionTitle = ({ children, className }: FormSectionTitleProps) => {
  return (
    <h3
      className={cn(
        'text-sm font-medium text-neutral-700 dark:text-neutral-300',
        className
      )}
    >
      {children}
    </h3>
  )
}
