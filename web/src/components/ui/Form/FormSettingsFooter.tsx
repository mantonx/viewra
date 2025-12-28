import { Button } from '@/components/ui/Button'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Check, X } from 'lucide-react'

type FormSettingsFooterProps = {
  /** Whether there are unsaved changes */
  hasChanges: boolean
  /** Handler for save action */
  onSave: () => void
  /** Handler for discard action */
  onDiscard: () => void
  /** Whether save is in progress */
  isSaving?: boolean
  /** Number of changed fields (shown in button if > 1) */
  changeCount?: number
  /** Custom save button label */
  saveLabel?: string
  /** Custom discard button label */
  discardLabel?: string
  /** Additional class name */
  className?: string
}

/**
 * Sticky footer for settings pages with save/discard buttons.
 * Only renders when there are unsaved changes.
 *
 * @example
 * <FormSettingsFooter
 *   hasChanges={hasChanges('category')}
 *   onSave={() => saveCategory('category')}
 *   onDiscard={() => discardCategory('category')}
 *   isSaving={isSaving('category')}
 *   changeCount={getChangeCount('category')}
 * />
 */
export const FormSettingsFooter = ({
  hasChanges,
  onSave,
  onDiscard,
  isSaving = false,
  changeCount,
  saveLabel = 'Save Changes',
  discardLabel = 'Discard',
  className,
}: FormSettingsFooterProps) => {
  if (!hasChanges) {
    return null
  }

  const showCount = changeCount !== undefined && changeCount > 1

  return (
    <div
      className={cn(
        'sticky bottom-4 flex items-center justify-end gap-3 p-4 rounded-xl',
        'bg-white dark:bg-neutral-900',
        'border border-neutral-200 dark:border-neutral-700',
        'shadow-lg',
        className
      )}
    >
      <span className={cn('text-sm mr-auto', text.secondary)}>
        You have unsaved changes
      </span>
      <Button variant="ghost" onClick={onDiscard} disabled={isSaving}>
        <X className="w-4 h-4 mr-1" />
        {discardLabel}
      </Button>
      <Button onClick={onSave} isLoading={isSaving}>
        <Check className="w-4 h-4 mr-1" />
        {saveLabel} {showCount && `(${changeCount})`}
      </Button>
    </div>
  )
}

export type { FormSettingsFooterProps }
