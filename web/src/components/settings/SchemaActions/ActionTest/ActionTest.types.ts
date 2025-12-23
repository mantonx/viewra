/**
 * Types for ActionTest component.
 */

import type { TestAction } from '@/lib/types/schema-actions'

export interface ActionTestProps {
  /** Action configuration from schema */
  action: TestAction
  /** Plugin ID for API calls */
  pluginId: string
  /** Additional class names */
  className?: string
}

export interface TestResult {
  success: boolean
  message?: string
  error?: string
}
