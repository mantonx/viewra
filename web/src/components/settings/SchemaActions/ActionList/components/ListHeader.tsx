import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui'
import type { ListHeaderProps } from './ListHeader.types'

export const ListHeader = ({ title, count, isLoading, onRefresh }: ListHeaderProps) => {
  return (
    <div className="flex items-center justify-between">
      <span className={cn('text-sm font-medium', text.primary)}>
        {title} ({count})
      </span>
      <Button variant="ghost" size="sm" onClick={onRefresh} disabled={isLoading}>
        <RefreshCw className={cn('w-3.5 h-3.5', isLoading && 'animate-spin')} />
      </Button>
    </div>
  )
}

ListHeader.displayName = 'ListHeader'
