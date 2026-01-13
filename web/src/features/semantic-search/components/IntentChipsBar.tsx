import { IntentChip, type IntentChipType } from './IntentChip'
import { cn } from '@/lib/utils'

interface IntentChipsBarProps {
  chips: IntentChipType[]
  onRemoveChip?: (chipId: string) => void
  onRefineChip?: (chipId: string, refinement: string) => void
  className?: string
}

/**
 * IntentChipsBar - Displays detected query intents as chips
 *
 * Shows users what the system understood from their search query.
 * Appears between the search bar and results.
 */
export const IntentChipsBar = ({
  chips,
  onRemoveChip,
  onRefineChip,
  className,
}: IntentChipsBarProps) => {
  if (!chips || chips.length === 0) {
    return null
  }

  return (
    <div
      className={cn(
        'flex items-center gap-2 px-6 py-3 rounded-lg',
        'bg-neutral-50/50 dark:bg-white/5',
        'border border-neutral-200/50 dark:border-white/10',
        'backdrop-blur-sm',
        className
      )}
    >
      <span className="text-sm font-medium text-neutral-600 dark:text-neutral-400 shrink-0">
        Understanding:
      </span>
      <div className="flex flex-wrap items-center gap-2">
        {chips.map((chip) => (
          <IntentChip
            key={chip.id}
            chip={chip}
            onRemove={onRemoveChip}
            onRefine={onRefineChip}
          />
        ))}
      </div>
    </div>
  )
}
