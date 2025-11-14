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
      <div ref={ref} className={cn('flex flex-col items-center gap-2', className)} {...props}>
        <Spinner size={size} className="text-blue-600" />
        {text && <p className="text-sm text-gray-600">{text}</p>}
      </div>
    )
  }
)

Loading.displayName = 'Loading'
