import { cn } from '@/lib/utils'
import type { ButtonHTMLAttributes } from 'react'
import { forwardRef } from 'react'
import { Spinner } from '@/components/ui/Spinner'
import { cva, type VariantProps } from 'class-variance-authority'

const buttonVariants = cva(
  // Base styles
  'cursor-pointer inline-flex items-center justify-center rounded font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 dark:focus-visible:ring-offset-neutral-900',
  {
    variants: {
      variant: {
        primary:
          'bg-blue-600 text-white hover:bg-blue-700 focus-visible:ring-blue-500 dark:bg-blue-600 dark:hover:bg-blue-500',
        secondary:
          'bg-neutral-100 text-neutral-700 hover:bg-neutral-200 focus-visible:ring-neutral-500 dark:bg-neutral-700 dark:text-neutral-100 dark:hover:bg-neutral-600',
        danger:
          'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500 dark:bg-red-600 dark:hover:bg-red-500',
        success:
          'bg-green-600 text-white hover:bg-green-700 focus-visible:ring-green-500 dark:bg-green-600 dark:hover:bg-green-500',
        ghost:
          'hover:bg-neutral-100 text-neutral-700 focus-visible:ring-neutral-500 dark:hover:bg-neutral-800 dark:text-neutral-100',
      },
      size: {
        sm: 'px-3 py-2.5 text-sm min-h-[44px]',
        md: 'px-4 py-3 text-base min-h-[44px]',
        lg: 'px-6 py-4 text-lg min-h-[48px]',
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
