import { cn } from '@/lib/utils'
import { HardDrive } from 'lucide-react'

/**
 * Badge indicating a provider runs locally (e.g., Ollama).
 */
export const LocalBadge = () => (
  <span
    className={cn(
      'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
      'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
    )}
  >
    <HardDrive className="w-2.5 h-2.5" />
    Local
  </span>
)
