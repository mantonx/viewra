/**
 * Hook for handling delete actions with confirmation.
 *
 * @example
 * ```tsx
 * const { pendingItem, isDeleting, confirmDelete, executeDelete, cancelDelete } = useDeleteAction(
 *   'ollama',
 *   { endpoint: '/models/:id' },
 *   { onSuccess: refresh }
 * )
 * ```
 */

import { useState, useCallback } from 'react'
import { pluginApi } from '@/lib/api/pluginApi'
import type { ListItem } from './useListData.types'
import type {
  DeleteActionConfig,
  UseDeleteActionOptions,
  UseDeleteActionReturn,
} from './useDeleteAction.types'

export const useDeleteAction = (
  pluginId: string,
  config: DeleteActionConfig,
  options: UseDeleteActionOptions = {}
): UseDeleteActionReturn => {
  const { onSuccess, onError } = options

  const [pendingItem, setPendingItem] = useState<ListItem | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const confirmDelete = useCallback((item: ListItem) => {
    setPendingItem(item)
  }, [])

  const cancelDelete = useCallback(() => {
    setPendingItem(null)
  }, [])

  const executeDelete = useCallback(async () => {
    if (!pendingItem) {
      return
    }

    setIsDeleting(true)
    try {
      // Replace :id placeholder with actual item id
      const endpoint = config.endpoint.replace(':id', String(pendingItem.id))
      await pluginApi.delete(pluginId, endpoint)
      onSuccess?.()
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err))
      onError?.(error)
    } finally {
      setIsDeleting(false)
      setPendingItem(null)
    }
  }, [pluginId, config.endpoint, pendingItem, onSuccess, onError])

  return {
    pendingItem,
    isDeleting,
    confirmDelete,
    executeDelete,
    cancelDelete,
  }
}

export type { DeleteActionConfig, UseDeleteActionOptions, UseDeleteActionReturn }
