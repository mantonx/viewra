/**
 * Types for ActionList component.
 */

import type { ListAction } from '@/lib/types/schema-actions'

export interface ActionListProps {
  /** Action configuration from schema */
  action: ListAction
  /** Plugin ID for API calls */
  pluginId: string
  /** Called when list should be refreshed externally */
  onRefresh?: () => void
  /** Called to show create action */
  onShowCreate?: (actionId: string) => void
  /** Additional class names */
  className?: string
}
