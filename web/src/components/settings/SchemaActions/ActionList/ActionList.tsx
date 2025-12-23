/**
 * ActionList - Renders a list of items from a plugin endpoint with actions.
 *
 * This component is schema-driven, using the `x-viewra-actions` extension
 * from the plugin's JSON Schema to configure its behavior.
 */

import { useState, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { Loading, EmptyState, Button } from '@/components/ui'
import { useListData, useDeleteAction, useStreamingAction } from '@/lib/hooks'
import { useToast } from '@/lib/hooks/useToast'
import { pluginApi } from '@/lib/api/pluginApi'
import type { ItemAction } from '@/lib/types/schema-actions'
import type { ListItem as ListItemData } from '@/lib/hooks/useListData.types'
import { ListHeader, ListItem, ItemActions, DeleteConfirmModal } from './components'
import type { ActionListProps } from './ActionList.types'

export const ActionList = ({
  action,
  pluginId,
  onRefresh,
  onShowCreate,
  className,
}: ActionListProps) => {
  const toast = useToast()

  // Data fetching
  const { items, isLoading, refresh } = useListData(pluginId, action.source.endpoint)

  // Find delete action config
  const deleteActionConfig = action.itemActions?.find((a) => a.type === 'delete')

  // Delete handling
  const {
    pendingItem: deleteItem,
    isDeleting,
    confirmDelete,
    executeDelete,
    cancelDelete,
  } = useDeleteAction(
    pluginId,
    {
      endpoint: deleteActionConfig?.endpoint || '',
      confirmTitle: deleteActionConfig?.confirm?.title,
      confirmMessage: deleteActionConfig?.confirm?.message,
    },
    {
      onSuccess: () => {
        toast.success('Item deleted')
        refresh()
        onRefresh?.()
      },
      onError: (err) => toast.error(`Failed to delete: ${err.message}`),
    }
  )

  // Streaming actions
  const {
    execute: executeStreaming,
    progress: streamingProgress,
    isStreaming,
    cancel: cancelStreaming,
  } = useStreamingAction(pluginId, {
    onComplete: () => {
      toast.success('Action completed')
      refresh()
      onRefresh?.()
    },
    onError: (err) => toast.error(`Failed: ${err.message}`),
  })

  // Track which item is currently streaming
  const [streamingItemId, setStreamingItemId] = useState<string | null>(null)

  // Track non-streaming action execution
  const [executingAction, setExecutingAction] = useState<{
    itemId: string
    actionId: string
  } | null>(null)

  // Handle refresh
  const handleRefresh = useCallback(() => {
    refresh()
    onRefresh?.()
  }, [refresh, onRefresh])

  // Handle action execution
  const handleAction = useCallback(
    async (itemAction: ItemAction, item: ListItemData) => {
      const itemId = String(item.id ?? item.name ?? '')

      if (itemAction.streaming) {
        setStreamingItemId(itemId)
        await executeStreaming(itemAction.endpoint, { model: item.id, name: item.name })
        setStreamingItemId(null)
      } else {
        setExecutingAction({ itemId, actionId: itemAction.id })
        try {
          await pluginApi.post(pluginId, itemAction.endpoint, { name: item.name, id: item.id })
          toast.success(`${itemAction.label} completed`)
          refresh()
          onRefresh?.()
        } catch (err) {
          toast.error(`Failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
        } finally {
          setExecutingAction(null)
        }
      }
    },
    [pluginId, executeStreaming, refresh, onRefresh, toast]
  )

  // Loading state
  if (isLoading) {
    return <Loading text="Loading..." />
  }

  // Empty state
  if (items.length === 0) {
    return (
      <EmptyState
        title={action.emptyState?.title || 'No items'}
        description={action.emptyState?.description}
        action={
          action.emptyState?.showCreate && onShowCreate ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onShowCreate(action.emptyState!.showCreate!)}
            >
              Add one
            </Button>
          ) : undefined
        }
      />
    )
  }

  return (
    <div className={cn('space-y-4', className)}>
      <ListHeader
        title={action.title}
        count={items.length}
        isLoading={isLoading}
        onRefresh={handleRefresh}
      />

      <div className="space-y-2">
        {items.map((item, idx) => {
          const itemId = String(item.id ?? item.name ?? idx)
          const isThisItemStreaming = streamingItemId === itemId

          return (
            <ListItem
              key={itemId}
              item={item}
              display={action.display}
              actions={
                action.itemActions && (
                  <ItemActions
                    item={item}
                    actions={action.itemActions}
                    streamingProgress={isThisItemStreaming ? streamingProgress : null}
                    isAnyStreaming={isStreaming}
                    executingActionId={
                      executingAction?.itemId === itemId ? executingAction.actionId : null
                    }
                    onAction={(a) => handleAction(a, item)}
                    onDelete={() => confirmDelete(item)}
                    onCancelStreaming={cancelStreaming}
                  />
                )
              }
            />
          )
        })}
      </div>

      {deleteActionConfig?.confirm && (
        <DeleteConfirmModal
          isOpen={deleteItem !== null}
          title={deleteActionConfig.confirm.title}
          message={deleteActionConfig.confirm.message}
          itemName={
            deleteItem
              ? String(deleteItem[action.display.primaryField] ?? deleteItem.id ?? '')
              : ''
          }
          isDeleting={isDeleting}
          onConfirm={executeDelete}
          onCancel={cancelDelete}
        />
      )}
    </div>
  )
}

ActionList.displayName = 'ActionList'
