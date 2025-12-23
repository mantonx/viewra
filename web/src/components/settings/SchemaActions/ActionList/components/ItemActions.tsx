import { Trash2, Download } from 'lucide-react'
import { Button, StreamingProgress } from '@/components/ui'
import type { ItemAction } from '@/lib/types/schema-actions'
import type { ItemActionsProps } from './ItemActions.types'

/** Check if an action should be shown based on showWhen condition */
const shouldShowAction = (action: ItemAction, item: Record<string, unknown>): boolean => {
  if (!action.showWhen) {
    return true
  }
  const { field, value } = action.showWhen
  return item[field] === value
}

export const ItemActions = ({
  item,
  actions,
  streamingProgress,
  isAnyStreaming,
  executingActionId,
  onAction,
  onDelete,
  onCancelStreaming,
}: ItemActionsProps) => {
  // If this item is streaming, show progress instead of actions
  if (streamingProgress) {
    const percent = streamingProgress.percent ??
      (streamingProgress.completed && streamingProgress.total
        ? Math.round((streamingProgress.completed / streamingProgress.total) * 100)
        : 0)

    return (
      <StreamingProgress
        variant="inline"
        status={streamingProgress.status}
        percent={percent}
        onCancel={onCancelStreaming}
      />
    )
  }

  return (
    <>
      {actions
        .filter((action) => shouldShowAction(action, item))
        .map((action) => {
          const isExecuting = executingActionId === action.id

          if (action.type === 'delete') {
            return (
              <Button
                key={action.id}
                variant="ghost"
                size="sm"
                onClick={onDelete}
                className="text-red-500 hover:text-red-600"
                disabled={isExecuting || isAnyStreaming}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            )
          }

          if (action.type === 'action') {
            return (
              <Button
                key={action.id}
                variant="secondary"
                size="sm"
                onClick={() => onAction(action)}
                disabled={isExecuting || isAnyStreaming}
                isLoading={isExecuting}
              >
                {!isExecuting && <Download className="w-3.5 h-3.5 mr-1" />}
                {action.label}
              </Button>
            )
          }

          return null
        })}
    </>
  )
}

ItemActions.displayName = 'ItemActions'
