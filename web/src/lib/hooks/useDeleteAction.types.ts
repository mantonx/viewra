/**
 * Types for useDeleteAction hook.
 */

import type { ListItem } from './useListData.types'

export interface DeleteActionConfig {
  /** Endpoint template (e.g., '/models/:id') */
  endpoint: string
  /** Confirmation dialog title */
  confirmTitle?: string
  /** Confirmation dialog message (supports {name} placeholder) */
  confirmMessage?: string
}

export interface UseDeleteActionOptions {
  /** Called after successful delete */
  onSuccess?: () => void
  /** Called on delete error */
  onError?: (error: Error) => void
}

export interface UseDeleteActionReturn {
  /** Item currently pending confirmation */
  pendingItem: ListItem | null
  /** Whether delete is in progress */
  isDeleting: boolean
  /** Request confirmation for an item */
  confirmDelete: (item: ListItem) => void
  /** Execute the pending delete */
  executeDelete: () => Promise<void>
  /** Cancel the pending delete */
  cancelDelete: () => void
}
