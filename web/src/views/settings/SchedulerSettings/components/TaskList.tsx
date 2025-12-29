/**
 * TaskList Component
 * Renders a flat list of tasks with subtle group headers
 */

import { Spinner } from '@/components/ui'
import { Settings, Puzzle } from 'lucide-react'
import type { TaskGroupInfo, TaskStatus } from '../SchedulerSettings.types'
import { TaskRow } from './TaskRow'

interface TaskListProps {
  groups: TaskGroupInfo[]
  isLoading: boolean
  error: Error | null
  expandedTasks: Set<string>
  onToggleTask: (taskId: string) => void
  // Actions
  onTriggerTask: (taskId: string) => void
  onEnableTask: (taskId: string) => void
  onDisableTask: (taskId: string) => void
  onEditSchedule: (task: TaskStatus) => void
  onViewHistory: (taskId: string) => void
  // Loading states
  isTriggeringTask: (taskId: string) => boolean
  isEnablingTask: (taskId: string) => boolean
  isDisablingTask: (taskId: string) => boolean
}

export const TaskList = ({
  groups,
  isLoading,
  error,
  expandedTasks,
  onToggleTask,
  onTriggerTask,
  onEnableTask,
  onDisableTask,
  onEditSchedule,
  onViewHistory,
  isTriggeringTask,
  isEnablingTask,
  isDisablingTask,
}: TaskListProps) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner size="lg" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="text-red-600 dark:text-red-400 mb-4">Failed to load scheduler tasks</p>
        <p className="text-sm text-neutral-500 dark:text-neutral-400">{error.message}</p>
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className="text-center py-12 text-neutral-500 dark:text-neutral-400">
        No tasks found matching your filters
      </div>
    )
  }

  return (
    <div className="divide-y divide-neutral-200 dark:divide-neutral-700">
      {groups.map((group) => (
        <div key={group.id}>
          {/* Simple group header */}
          <div className="flex items-center gap-2 px-4 py-2 bg-neutral-50 dark:bg-neutral-800/50">
            {group.source === 'plugin' ? (
              <Puzzle className="w-4 h-4 text-purple-500" />
            ) : (
              <Settings className="w-4 h-4 text-neutral-400" />
            )}
            <span className="text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wide">
              {group.name}
            </span>
          </div>

          {/* Tasks in this group */}
          {group.tasks.map((task) => (
            <TaskRow
              key={task.id}
              task={task}
              isExpanded={expandedTasks.has(task.id)}
              onToggleExpand={() => onToggleTask(task.id)}
              onTrigger={() => onTriggerTask(task.id)}
              onEnable={() => onEnableTask(task.id)}
              onDisable={() => onDisableTask(task.id)}
              onEditSchedule={() => onEditSchedule(task)}
              onViewHistory={() => onViewHistory(task.id)}
              isTriggering={isTriggeringTask(task.id)}
              isEnabling={isEnablingTask(task.id)}
              isDisabling={isDisablingTask(task.id)}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
