import { cn } from '@/lib/utils'
import { RotateCcw } from 'lucide-react'

/**
 * Badge indicating a setting requires server restart to take effect.
 */
export const RestartBadge = () => (
  <span
    className={cn(
      'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
      'bg-orange-100 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400'
    )}
  >
    <RotateCcw className="w-2.5 h-2.5" />
    restart required
  </span>
)
