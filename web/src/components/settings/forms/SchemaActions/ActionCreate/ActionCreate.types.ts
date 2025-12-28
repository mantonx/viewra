/**
 * Types for ActionCreate component.
 */

import type { CreateAction } from '@/lib/types/schema-actions'

export interface ActionCreateProps {
  /** Action configuration from schema */
  action: CreateAction
  /** Plugin ID for API calls */
  pluginId: string
  /** Called on successful creation */
  onSuccess?: () => void
  /** Additional class names */
  className?: string
}
