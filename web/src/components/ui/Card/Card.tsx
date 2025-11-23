import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'bordered' | 'elevated'
}

export const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant = 'default', children, ...props }, ref) => {
    const variants = {
      default: 'bg-white dark:bg-neutral-900 rounded-lg shadow dark:shadow-neutral-950/50',
      bordered: 'bg-white dark:bg-neutral-900 rounded-lg border border-neutral-200 dark:border-neutral-800',
      elevated: 'bg-white dark:bg-neutral-900 rounded-lg shadow-lg dark:shadow-neutral-950/50',
    }

    return (
      <div ref={ref} className={cn(variants[variant], className)} {...props}>
        {children}
      </div>
    )
  }
)

Card.displayName = 'Card'

export const CardHeader = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div ref={ref} className={cn('p-4 border-b border-neutral-200 dark:border-neutral-800', className)} {...props} />
  }
)

CardHeader.displayName = 'CardHeader'

export const CardContent = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div ref={ref} className={cn('p-4', className)} {...props} />
  }
)

CardContent.displayName = 'CardContent'

export const CardFooter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div ref={ref} className={cn('p-4 border-t border-neutral-200 dark:border-neutral-800', className)} {...props} />
  }
)

CardFooter.displayName = 'CardFooter'
