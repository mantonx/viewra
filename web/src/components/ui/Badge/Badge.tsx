import { forwardRef } from 'react'
import { cn } from '@/lib/utils'
import { cva, type VariantProps } from 'class-variance-authority'
import type { BadgeProps } from './Badge.types'

const badgeVariants = cva(
  'inline-flex items-center font-medium rounded',
  {
    variants: {
      color: {
        blue: 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-400',
        green: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-400',
        yellow: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-400',
        red: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-400',
        purple: 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-400',
        gray: 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-400',
        emerald: 'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400',
        primary: 'bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-400',
      },
      size: {
        sm: 'px-1.5 py-0.5 text-[10px]',
        md: 'px-2 py-1 text-xs',
      },
    },
    defaultVariants: {
      color: 'gray',
      size: 'sm',
    },
  }
)

export const Badge = forwardRef<HTMLSpanElement, BadgeProps & VariantProps<typeof badgeVariants>>(
  ({ children, color, size, className, ...props }, ref) => {
    return (
      <span
        ref={ref}
        className={cn(badgeVariants({ color, size }), className)}
        {...props}
      >
        {children}
      </span>
    )
  }
)

Badge.displayName = 'Badge'
