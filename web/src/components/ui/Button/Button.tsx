import { cn } from '@/lib/utils'
import type { ButtonHTMLAttributes } from 'react'
import { forwardRef } from 'react'
import { Spinner } from '@/components/ui/Spinner'
import { cva, type VariantProps } from 'class-variance-authority'

const buttonVariants = cva(
  // Base styles - refined transitions
  'cursor-pointer inline-flex items-center justify-center rounded-lg font-medium transition-all duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 dark:focus-visible:ring-offset-neutral-900',
  {
    variants: {
      variant: {
        primary:
          'bg-primary-600 text-white hover:bg-primary-500 hover:shadow-lg hover:shadow-primary-500/20 active:scale-[0.98] focus-visible:ring-primary-500 dark:bg-primary-600 dark:hover:bg-primary-500 dark:hover:shadow-primary-500/25',
        secondary:
          'bg-neutral-100 text-neutral-700 hover:bg-neutral-200 focus-visible:ring-neutral-400 dark:bg-white/10 dark:text-neutral-100 dark:hover:bg-white/15 dark:border dark:border-white/5',
        danger:
          'bg-error-600 text-white hover:bg-error-500 hover:shadow-lg hover:shadow-error-500/20 focus-visible:ring-error-500 dark:bg-error-600 dark:hover:bg-error-500',
        success:
          'bg-success-600 text-white hover:bg-success-500 hover:shadow-lg hover:shadow-success-500/20 focus-visible:ring-success-500 dark:bg-success-600 dark:hover:bg-success-500',
        ghost:
          'hover:bg-neutral-100 text-neutral-700 focus-visible:ring-neutral-400 dark:hover:bg-white/5 dark:text-neutral-100',
        glass:
          'bg-white/90 text-neutral-900 border border-neutral-200/50 hover:bg-white hover:shadow-lg hover:shadow-black/5 active:scale-[0.98] focus-visible:ring-neutral-400 dark:bg-white/10 dark:text-white dark:border-white/10 dark:hover:bg-white/15 dark:hover:shadow-lg dark:hover:shadow-black/30',
      },
      size: {
        sm: 'px-3 py-2 text-sm min-h-[36px]',
        md: 'px-4 py-2.5 text-sm min-h-[40px]',
        lg: 'px-6 py-3 text-base min-h-[44px]',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'md',
    },
  }
)

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  isLoading?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, isLoading, disabled, children, type = 'button', ...props }, ref) => {
    return (
      <button
        ref={ref}
        type={type}
        className={cn(buttonVariants({ variant, size }), className)}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading && <Spinner size="sm" className="mr-2" />}
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'
