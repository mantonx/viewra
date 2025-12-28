/**
 * Types for ListItem component.
 */

import type { ReactNode } from 'react'
import type { ActionDisplay } from '@/lib/types/schema-actions'
import type { ListItem as ListItemData } from '@/lib/hooks/useListData.types'

export interface ListItemProps {
  /** Item data */
  item: ListItemData
  /** Display configuration */
  display: ActionDisplay
  /** Actions slot */
  actions?: ReactNode
}
