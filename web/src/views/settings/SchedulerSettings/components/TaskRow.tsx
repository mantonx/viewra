/**
 * TaskRow Component
 * A clean, simple task row with inline expansion
 */

import { cn } from '@/lib/utils'
import { Button } from '@/components/ui'
import { cronToShortDescription, cronToReadable } from '@/lib/utils/cron'
import { formatDistanceToNow } from 'date-fns'
import type { TaskStatus } from '../SchedulerSettings.types'
import { Play, Clock, History, Pencil, ChevronRight } from 'lucide-react'

interface TaskRowProps {
  task: TaskStatus
  isExpanded: boolean
  onToggleExpand: () => void
  onTrigger: () => void
  onEnable: () => void
  onDisable: () => void
  onEditSchedule: () => void
  onViewHistory: () => void
  isTriggering: boolean
  isEnabling: boolean
  isDisabling: boolean
}

const StatusDot = ({ task }: { task: TaskStatus }) => {
  if (task.is_running) {
    return (
      <span className="relative flex h-2 w-2">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500" />
      </span>
    )
  }
  if (!task.enabled) {
    return <span className="h-2 w-2 rounded-full bg-neutral-300 dark:bg-neutral-600" />
  }
  if (task.last_success === false) {
    return <span className="h-2 w-2 rounded-full bg-red-500" />
  }
  return <span className="h-2 w-2 rounded-full bg-green-500" />
}

const formatRelativeTime = (dateStr?: string): string => {
  if (!dateStr) {
    return 'Never'
  }
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true })
  } catch {
    return '-'
  }
}

export const TaskRow = ({
  task,
  isExpanded,
  onToggleExpand,
  onTrigger,
  onEnable,
  onDisable,
  onEditSchedule,
  onViewHistory,
  isTriggering,
  isEnabling,
  isDisabling,
}: TaskRowProps) => {
  return (
    <div
      className={cn(
        'border-b border-neutral-100 dark:border-neutral-800 last:border-b-0',
        !task.enabled && 'opacity-50'
      )}
    >
      {/* Main row - always visible */}
      <div
        className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-neutral-50 dark:hover:bg-neutral-800/30 transition-colors"
        onClick={onToggleExpand}
      >
        <ChevronRight
          className={cn(
            'w-4 h-4 text-neutral-400 transition-transform',
            isExpanded && 'rotate-90'
          )}
        />

        <StatusDot task={task} />

        <div className="flex-1 min-w-0">
          <span className="font-medium text-neutral-900 dark:text-neutral-100">{task.name}</span>
        </div>

        <div className="flex items-center gap-4 text-sm text-neutral-500 dark:text-neutral-400">
          <span className="hidden sm:flex items-center gap-1">
            <Clock className="w-3.5 h-3.5" />
            {cronToShortDescription(task.schedule)}
          </span>
          <span className="hidden md:block">{formatRelativeTime(task.last_run)}</span>
        </div>

        <Button
          variant="ghost"
          size="sm"
          className="p-1.5"
          onClick={(e) => {
            e.stopPropagation()
            onTrigger()
          }}
          disabled={!task.enabled || task.is_running}
          isLoading={isTriggering}
          title="Run now"
        >
          <Play className="w-4 h-4" />
        </Button>
      </div>

      {/* Expanded content */}
      {isExpanded && (
        <div className="px-4 pb-4 pt-1 ml-7 border-l-2 border-neutral-200 dark:border-neutral-700 space-y-3">
          <p className="text-sm text-neutral-600 dark:text-neutral-400">{task.description}</p>

          <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
            <div>
              <span className="text-neutral-500 dark:text-neutral-500">Schedule:</span>{' '}
              <span className="text-neutral-900 dark:text-neutral-100">{cronToReadable(task.schedule)}</span>
            </div>
            <div>
              <span className="text-neutral-500 dark:text-neutral-500">Last run:</span>{' '}
              <span className="text-neutral-900 dark:text-neutral-100">{formatRelativeTime(task.last_run)}</span>
            </div>
            <div>
              <span className="text-neutral-500 dark:text-neutral-500">Next run:</span>{' '}
              <span className="text-neutral-900 dark:text-neutral-100">
                {task.enabled ? formatRelativeTime(task.next_run) : 'Disabled'}
              </span>
            </div>
          </div>

          {task.last_error && (
            <div className="text-sm">
              <span className="text-red-600 dark:text-red-400 font-medium">Last error: </span>
              <span className="text-red-600 dark:text-red-400 font-mono text-xs">{task.last_error}</span>
            </div>
          )}

          <div className="flex flex-wrap gap-2 pt-1">
            <Button
              variant="primary"
              size="sm"
              onClick={onTrigger}
              disabled={!task.enabled || task.is_running}
              isLoading={isTriggering}
            >
              <Play className="w-3.5 h-3.5 mr-1" />
              Run Now
            </Button>

            <Button variant="secondary" size="sm" onClick={onEditSchedule}>
              <Pencil className="w-3.5 h-3.5 mr-1" />
              Edit
            </Button>

            <Button variant="secondary" size="sm" onClick={onViewHistory}>
              <History className="w-3.5 h-3.5 mr-1" />
              History
            </Button>

            {task.enabled ? (
              <Button variant="ghost" size="sm" onClick={onDisable} isLoading={isDisabling}>
                Disable
              </Button>
            ) : (
              <Button variant="ghost" size="sm" onClick={onEnable} isLoading={isEnabling}>
                Enable
              </Button>
            )}
          </div>

          <div className="text-xs text-neutral-400 dark:text-neutral-500 font-mono pt-1">
            ID: {task.id} | Cron: {task.schedule}
          </div>
        </div>
      )}
    </div>
  )
}
