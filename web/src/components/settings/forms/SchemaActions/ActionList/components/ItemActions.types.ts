/**
 * Types for ItemActions component.
 */

import type { ItemAction } from '@/lib/types/schema-actions'
import type { StreamingProgress } from '@/lib/hooks/useStreamingAction.types'
import type { ListItem } from '@/lib/hooks/useListData.types'

export interface ItemActionsProps {
  /** Item data */
  item: ListItem
  /** Available actions */
  actions: ItemAction[]
  /** Currently streaming state (for this item) */
  streamingProgress?: StreamingProgress | null
  /** Whether any action is streaming */
  isAnyStreaming: boolean
  /** Currently executing action ID (non-streaming) */
  executingActionId?: string | null
  /** Execute an action */
  onAction: (action: ItemAction) => void
  /** Request delete confirmation */
  onDelete: () => void
  /** Cancel streaming */
  onCancelStreaming?: () => void
}
