import { useState, useCallback, useEffect, useRef } from 'react'
import { Button, Modal, ModalContent, ModalFooter } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Trash2, RefreshCw, AlertCircle, Loader2, Download, X } from 'lucide-react'
import type { ListAction, ActionBadge, ItemAction, StreamingProgress } from '@/lib/types/schema-actions'

type ListItem = Record<string, unknown>

type StreamingState = {
  itemId: string
  actionId: string
  progress: StreamingProgress
  abortController: AbortController
}

type ActionListProps = {
  action: ListAction
  pluginId: string
  onRefresh?: () => void
  onShowCreate?: (actionId: string) => void
  className?: string
}

/** Check if an item action should be shown based on showWhen condition */
const shouldShowAction = (itemAction: ItemAction, item: ListItem): boolean => {
  if (!itemAction.showWhen) {
    return true
  }
  const { field, value } = itemAction.showWhen
  return item[field] === value
}

/** Badge colors mapping */
const badgeColors: Record<ActionBadge['color'], string> = {
  blue: 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-400',
  green: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-400',
  yellow: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-400',
  red: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-400',
  purple: 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-400',
  gray: 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-400',
  emerald: 'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400',
}

export const ActionList = ({
  action,
  pluginId,
  onRefresh,
  onShowCreate,
  className,
}: ActionListProps) => {
  const toast = useToast()
  const [items, setItems] = useState<ListItem[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [deleteConfirm, setDeleteConfirm] = useState<ListItem | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  // Track which item+action is currently executing (non-streaming)
  const [executingAction, setExecutingAction] = useState<{ itemId: string; actionId: string } | null>(null)
  // Track streaming actions with progress
  const [streamingState, setStreamingState] = useState<StreamingState | null>(null)
  const readerRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null)

  const fetchItems = useCallback(async () => {
    setIsLoading(true)
    try {
      const response = await fetch(`/api/plugin/${pluginId}${action.source.endpoint}`, {
        credentials: 'include',
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const data = await response.json()
      setItems(data.items || [])
    } catch (err) {
      toast.error(`Failed to load items: ${err instanceof Error ? err.message : 'Unknown error'}`)
      setItems([])
    } finally {
      setIsLoading(false)
    }
  }, [pluginId, action.source.endpoint, toast])

  // Initial fetch
  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  const handleDelete = useCallback(
    async (item: ListItem) => {
      const deleteAction = action.itemActions?.find((a) => a.type === 'delete')
      if (!deleteAction) {
        return
      }

      setIsDeleting(true)
      try {
        // Replace :id placeholder with actual item id
        const endpoint = deleteAction.endpoint.replace(':id', String(item.id))
        const response = await fetch(`/api/plugin/${pluginId}${endpoint}`, {
          method: 'DELETE',
          credentials: 'include',
        })

        if (!response.ok) {
          const data = await response.json()
          throw new Error(data.error || `HTTP ${response.status}`)
        }

        toast.success('Item deleted')
        fetchItems()
        onRefresh?.()
      } catch (err) {
        toast.error(`Failed to delete: ${err instanceof Error ? err.message : 'Unknown error'}`)
      } finally {
        setIsDeleting(false)
        setDeleteConfirm(null)
      }
    },
    [action.itemActions, pluginId, toast, fetchItems, onRefresh]
  )

  const handleRefresh = useCallback(() => {
    fetchItems()
    onRefresh?.()
  }, [fetchItems, onRefresh])

  /** Execute a streaming item action with SSE progress */
  const handleStreamingAction = useCallback(
    async (itemAction: ItemAction, item: ListItem) => {
      const itemId = String(item.id ?? item.name ?? '')
      const abortController = new AbortController()

      setStreamingState({
        itemId,
        actionId: itemAction.id,
        progress: { status: 'Starting...' },
        abortController,
      })

      try {
        const response = await fetch(`/api/plugin/${pluginId}${itemAction.endpoint}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ model: item.id, name: item.name }),
          signal: abortController.signal,
        })

        if (!response.ok || !response.body) {
          throw new Error(`HTTP ${response.status}`)
        }

        const reader = response.body.getReader()
        readerRef.current = reader
        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() ?? ''

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const data = JSON.parse(line.slice(6)) as StreamingProgress
                setStreamingState((prev) =>
                  prev ? { ...prev, progress: data } : null
                )

                if (data.error) {
                  toast.error(data.error)
                  break
                }

                if (data.done) {
                  toast.success(`${itemAction.label} completed`)
                  fetchItems()
                  onRefresh?.()
                }
              } catch {
                // Ignore parse errors for incomplete JSON
              }
            }
          }
        }
      } catch (err) {
        if ((err as Error).name !== 'AbortError') {
          toast.error(`Failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
        }
      } finally {
        readerRef.current = null
        setStreamingState(null)
      }
    },
    [pluginId, toast, fetchItems, onRefresh]
  )

  /** Cancel a streaming action */
  const handleCancelStreaming = useCallback(() => {
    if (streamingState) {
      streamingState.abortController.abort()
      readerRef.current?.cancel()
      toast.info('Cancelled')
    }
  }, [streamingState, toast])

  /** Execute an item action (like Pull for models) */
  const handleItemAction = useCallback(
    async (itemAction: ItemAction, item: ListItem) => {
      // Use streaming handler if action is marked as streaming
      if (itemAction.streaming) {
        return handleStreamingAction(itemAction, item)
      }

      const itemId = String(item.id ?? item.name ?? '')
      setExecutingAction({ itemId, actionId: itemAction.id })
      try {
        const response = await fetch(`/api/plugin/${pluginId}${itemAction.endpoint}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ name: item.name, id: item.id }),
        })

        if (!response.ok) {
          const data = await response.json()
          throw new Error(data.error || `HTTP ${response.status}`)
        }

        toast.success(`${itemAction.label} completed`)
        fetchItems()
        onRefresh?.()
      } catch (err) {
        toast.error(`Failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
      } finally {
        setExecutingAction(null)
      }
    },
    [pluginId, toast, fetchItems, onRefresh, handleStreamingAction]
  )

  const renderBadges = (item: ListItem) => {
    if (!action.display.badges) {
      return null
    }

    return action.display.badges
      .filter((badge) => {
        const fieldValue = item[badge.field]
        // If value is specified, match it; otherwise check truthiness
        return badge.value !== undefined ? fieldValue === badge.value : Boolean(fieldValue)
      })
      .map((badge, idx) => (
        <span
          key={idx}
          className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium', badgeColors[badge.color])}
        >
          {badge.label}
        </span>
      ))
  }

  /** Render action buttons for an item */
  const renderItemActions = (item: ListItem) => {
    if (!action.itemActions) {
      return null
    }

    const itemId = String(item.id ?? item.name ?? '')
    const isStreaming = streamingState?.itemId === itemId

    // If this item is streaming, show progress instead of actions
    if (isStreaming) {
      const { progress } = streamingState
      const percent = progress.percent ?? (progress.completed && progress.total
        ? Math.round((progress.completed / progress.total) * 100)
        : 0)

      return (
        <div className="flex items-center gap-2 min-w-[180px]">
          <div className="flex-1">
            <div className="flex items-center justify-between mb-1">
              <span className={cn('text-[10px]', text.tertiary)}>
                {progress.status || 'Downloading...'}
              </span>
              <span className={cn('text-[10px] font-medium', text.secondary)}>
                {percent}%
              </span>
            </div>
            <div className="h-1.5 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
              <div
                className="h-full bg-primary-500 transition-all duration-300"
                style={{ width: `${percent}%` }}
              />
            </div>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleCancelStreaming}
            className="text-neutral-400 hover:text-neutral-600"
          >
            <X className="w-3.5 h-3.5" />
          </Button>
        </div>
      )
    }

    return action.itemActions
      .filter((itemAction) => shouldShowAction(itemAction, item))
      .map((itemAction) => {
        const isExecuting =
          executingAction?.itemId === itemId && executingAction?.actionId === itemAction.id
        const anyStreaming = streamingState !== null

        if (itemAction.type === 'delete') {
          return (
            <Button
              key={itemAction.id}
              variant="ghost"
              size="sm"
              onClick={() => setDeleteConfirm(item)}
              className="text-red-500 hover:text-red-600"
              disabled={isExecuting || anyStreaming}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          )
        }

        if (itemAction.type === 'action') {
          return (
            <Button
              key={itemAction.id}
              variant="secondary"
              size="sm"
              onClick={() => handleItemAction(itemAction, item)}
              disabled={isExecuting || anyStreaming}
              isLoading={isExecuting}
            >
              {!isExecuting && <Download className="w-3.5 h-3.5 mr-1" />}
              {itemAction.label}
            </Button>
          )
        }

        return null
      })
  }

  // Find delete action for the confirmation modal
  const deleteAction = action.itemActions?.find((a) => a.type === 'delete')

  return (
    <div className={cn('space-y-4', className)}>
      {/* Header with refresh button */}
      <div className="flex items-center justify-between">
        <span className={cn('text-sm font-medium', text.primary)}>
          {action.title} ({items.length})
        </span>
        <Button variant="ghost" size="sm" onClick={handleRefresh} disabled={isLoading}>
          <RefreshCw className={cn('w-3.5 h-3.5', isLoading && 'animate-spin')} />
        </Button>
      </div>

      {/* Loading state */}
      {isLoading ? (
        <div className="flex items-center justify-center py-4">
          <Loader2 className={cn('w-5 h-5 animate-spin', text.tertiary)} />
        </div>
      ) : items.length === 0 ? (
        /* Empty state */
        <div
          className={cn(
            'flex flex-col items-center justify-center py-4 px-3 rounded-lg',
            'bg-neutral-50 dark:bg-neutral-900/50',
            'border border-dashed border-neutral-200 dark:border-neutral-700'
          )}
        >
          <AlertCircle className={cn('w-5 h-5 mb-2', text.tertiary)} />
          <p className={cn('text-sm text-center', text.secondary)}>
            {action.emptyState?.title || 'No items'}
          </p>
          {action.emptyState?.description && (
            <p className={cn('text-xs text-center mt-1', text.tertiary)}>
              {action.emptyState.description}
            </p>
          )}
          {action.emptyState?.showCreate && onShowCreate && (
            <Button
              variant="ghost"
              size="sm"
              className="mt-3"
              onClick={() => {
                const createId = action.emptyState?.showCreate
                if (createId) {
                  onShowCreate(createId)
                }
              }}
            >
              Add one
            </Button>
          )}
        </div>
      ) : (
        /* Items list */
        <div className="space-y-2">
          {items.map((item, idx) => (
            <div
              key={String(item.id ?? item.name) || idx}
              className={cn(
                'py-3 px-3 rounded-lg flex items-center justify-between gap-3',
                'bg-neutral-50 dark:bg-neutral-900/50'
              )}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={cn('text-sm font-medium', text.primary)}>
                    {String(item[action.display.primaryField] || item.id)}
                  </span>
                  {action.display.secondaryField &&
                    item[action.display.secondaryField] !== null &&
                    item[action.display.secondaryField] !== undefined && (
                      <span className={cn('text-xs', text.tertiary)}>
                        {String(item[action.display.secondaryField])}
                      </span>
                    )}
                  {renderBadges(item)}
                </div>
              </div>
              <div className="flex items-center gap-1">{renderItemActions(item)}</div>
            </div>
          ))}
        </div>
      )}

      {/* Delete confirmation modal */}
      {deleteAction?.confirm && (
        <Modal
          isOpen={deleteConfirm !== null}
          onClose={() => setDeleteConfirm(null)}
          title={deleteAction.confirm.title}
        >
          <ModalContent>
            <div className="flex items-start gap-4">
              <div
                className={cn(
                  'p-2 rounded-full',
                  'bg-red-100 dark:bg-red-900/50',
                  'text-red-600 dark:text-red-400'
                )}
              >
                <Trash2 className="w-5 h-5" />
              </div>
              <div>
                <p className={cn('text-sm', text.secondary)}>
                  {deleteAction.confirm.message.replace(
                    '{name}',
                    deleteConfirm
                      ? String(deleteConfirm[action.display.primaryField] ?? deleteConfirm.id ?? '')
                      : ''
                  )}
                </p>
              </div>
            </div>
          </ModalContent>
          <ModalFooter>
            <Button variant="ghost" onClick={() => setDeleteConfirm(null)}>
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => deleteConfirm && handleDelete(deleteConfirm)}
              isLoading={isDeleting}
            >
              <Trash2 className="w-4 h-4 mr-1" />
              Delete
            </Button>
          </ModalFooter>
        </Modal>
      )}
    </div>
  )
}
