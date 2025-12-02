import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'
import { forwardRef } from 'react'
import { Spinner } from '@/components/ui/Spinner'

export interface LoadingProps extends HTMLAttributes<HTMLDivElement> {
  size?: 'sm' | 'md' | 'lg'
  text?: string
}

export const Loading = forwardRef<HTMLDivElement, LoadingProps>(
  ({ className, size = 'md', text, ...props }, ref) => {
    return (
      <div ref={ref} className={cn('flex flex-col items-center gap-3', className)} {...props}>
        <Spinner size={size} className="text-primary-600 dark:text-primary-400" />
        {text && <p className="text-sm text-neutral-500 dark:text-neutral-400">{text}</p>}
      </div>
    )
  }
)

Loading.displayName = 'Loading'
