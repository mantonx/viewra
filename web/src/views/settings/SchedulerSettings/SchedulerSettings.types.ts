/**
 * Scheduler Settings Types
 */

import type { TaskStatus, TaskExecution, ExecutionStatus, TriggeredBy } from '@/lib/types/scheduler'

// Re-export API types for convenience
export type { TaskStatus, TaskExecution, ExecutionStatus, TriggeredBy }

/**
 * Filter tab options
 */
export type FilterTab = 'all' | 'system' | 'plugins' | 'disabled'

/**
 * Tab counts for display
 */
export interface TabCounts {
  all: number
  system: number
  plugins: number
  disabled: number
}

/**
 * Group display info
 */
export interface TaskGroupInfo {
  id: string
  name: string
  source: 'internal' | 'plugin'
  tasks: TaskStatus[]
}

/**
 * Friendly names for system task groups
 */
export const GROUP_DISPLAY_NAMES: Record<string, string> = {
  cleanup: 'Cleanup Tasks',
  transcode: 'Transcode Tasks',
  auth: 'Authentication',
}

/**
 * Get display name for a group
 */
export const getGroupDisplayName = (sourceId: string, source: string): string => {
  if (source === 'plugin') {
    // Capitalize first letter of plugin name
    return `Plugin: ${sourceId.charAt(0).toUpperCase()}${sourceId.slice(1)}`
  }
  return GROUP_DISPLAY_NAMES[sourceId] || sourceId
}

/**
 * Loading states for task operations
 */
export interface TaskLoadingStates {
  trigger: Set<string>
  enable: Set<string>
  disable: Set<string>
}
