import { forwardRef } from 'react'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { AlertCircle } from 'lucide-react'
import type { EmptyStateProps } from './EmptyState.types'

const sizeClasses = {
  sm: {
    container: 'py-4 px-3',
    icon: 'w-5 h-5 mb-2',
    title: 'text-sm',
    description: 'text-xs mt-1',
    action: 'mt-3',
  },
  md: {
    container: 'py-6 px-4',
    icon: 'w-8 h-8 mb-3',
    title: 'text-base',
    description: 'text-sm mt-1.5',
    action: 'mt-4',
  },
  lg: {
    container: 'py-8 px-6',
    icon: 'w-10 h-10 mb-4',
    title: 'text-lg',
    description: 'text-base mt-2',
    action: 'mt-5',
  },
}

export const EmptyState = forwardRef<HTMLDivElement, EmptyStateProps>(
  (
    {
      icon,
      title,
      description,
      action,
      variant = 'dashed',
      size = 'sm',
      className,
    },
    ref
  ) => {
    const sizes = sizeClasses[size]
    const IconElement = icon ?? <AlertCircle className={cn(sizes.icon, text.tertiary)} />

    return (
      <div
        ref={ref}
        className={cn(
          'flex flex-col items-center justify-center rounded-lg',
          'bg-neutral-50 dark:bg-neutral-900/50',
          variant === 'dashed' && 'border border-dashed border-neutral-200 dark:border-neutral-700',
          sizes.container,
          className
        )}
      >
        {icon !== null && (
          <div className={cn(sizes.icon, text.tertiary)}>
            {IconElement}
          </div>
        )}
        <p className={cn('text-center font-medium', sizes.title, text.secondary)}>
          {title}
        </p>
        {description && (
          <p className={cn('text-center', sizes.description, text.tertiary)}>
            {description}
          </p>
        )}
        {action && (
          <div className={sizes.action}>
            {action}
          </div>
        )}
      </div>
    )
  }
)

EmptyState.displayName = 'EmptyState'
