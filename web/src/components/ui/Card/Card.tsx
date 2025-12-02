import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'
import { forwardRef } from 'react'

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'bordered' | 'elevated' | 'glass'
}

export const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant = 'default', children, ...props }, ref) => {
    const variants = {
      default: cn(
        'bg-white dark:bg-neutral-900',
        'shadow dark:shadow-neutral-950/50',
        'rounded-xl'
      ),
      bordered: cn(
        'bg-white/80 dark:bg-white/5',
        'border border-neutral-200/50 dark:border-white/10',
        'rounded-xl backdrop-blur-sm'
      ),
      elevated: cn(
        'bg-white dark:bg-neutral-900',
        'shadow-lg dark:shadow-neutral-950/50',
        'rounded-xl'
      ),
      glass: cn(
        'bg-white/50 dark:bg-white/5',
        'border border-neutral-200/50 dark:border-white/10',
        'rounded-xl backdrop-blur-md',
        'shadow-lg shadow-neutral-500/5 dark:shadow-neutral-950/20'
      ),
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
    return (
      <div
        ref={ref}
        className={cn(
          'p-4 border-b border-neutral-200/50 dark:border-white/10',
          className
        )}
        {...props}
      />
    )
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
    return (
      <div
        ref={ref}
        className={cn(
          'p-4 border-t border-neutral-200/50 dark:border-white/10',
          className
        )}
        {...props}
      />
    )
  }
)

CardFooter.displayName = 'CardFooter'
