/**
 * TaskHistoryModal Component
 * Displays execution history for a scheduler task
 */

import { Modal, ModalContent, ModalFooter, Badge, Button, Spinner } from '@/components/ui'
import { format, formatDuration, intervalToDuration } from 'date-fns'
import type { TaskExecution, ExecutionStatus, TriggeredBy } from '../SchedulerSettings.types'

interface TaskHistoryModalProps {
  isOpen: boolean
  onClose: () => void
  taskName: string
  history: TaskExecution[]
  isLoading: boolean
}

const STATUS_COLORS: Record<ExecutionStatus, 'green' | 'red' | 'blue' | 'yellow' | 'gray' | 'purple'> = {
  pending: 'gray',
  running: 'blue',
  completed: 'green',
  failed: 'red',
  cancelled: 'yellow',
  skipped: 'gray',
  interrupted: 'yellow',
}

const TRIGGER_LABELS: Record<TriggeredBy, string> = {
  schedule: 'Scheduled',
  manual: 'Manual',
  retry: 'Retry',
  dependency: 'Dependency',
}

const formatDurationMs = (ms: number): string => {
  if (ms < 1000) {
    return `${ms}ms`
  }
  const duration = intervalToDuration({ start: 0, end: ms })
  return formatDuration(duration, { format: ['hours', 'minutes', 'seconds'], zero: false }) || '< 1s'
}

const formatDateTime = (dateStr?: string): string => {
  if (!dateStr) {
    return '-'
  }
  try {
    return format(new Date(dateStr), 'MMM d, yyyy h:mm:ss a')
  } catch {
    return '-'
  }
}

export const TaskHistoryModal = ({
  isOpen,
  onClose,
  taskName,
  history,
  isLoading,
}: TaskHistoryModalProps) => {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`History: ${taskName}`} size="lg">
      <ModalContent>
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Spinner size="lg" />
          </div>
        ) : history.length === 0 ? (
          <div className="text-center py-12 text-neutral-500 dark:text-neutral-400">
            No execution history available
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-neutral-200 dark:border-neutral-700">
                  <th className="text-left py-3 px-2 font-medium text-neutral-500 dark:text-neutral-400">
                    Status
                  </th>
                  <th className="text-left py-3 px-2 font-medium text-neutral-500 dark:text-neutral-400">
                    Trigger
                  </th>
                  <th className="text-left py-3 px-2 font-medium text-neutral-500 dark:text-neutral-400">
                    Started
                  </th>
                  <th className="text-left py-3 px-2 font-medium text-neutral-500 dark:text-neutral-400">
                    Duration
                  </th>
                  <th className="text-left py-3 px-2 font-medium text-neutral-500 dark:text-neutral-400">
                    Attempt
                  </th>
                </tr>
              </thead>
              <tbody>
                {history.map((execution) => (
                  <tr
                    key={execution.id}
                    className="border-b border-neutral-100 dark:border-neutral-800 hover:bg-neutral-50 dark:hover:bg-neutral-800/50"
                  >
                    <td className="py-3 px-2">
                      <Badge color={STATUS_COLORS[execution.status]} size="sm">
                        {execution.status}
                      </Badge>
                    </td>
                    <td className="py-3 px-2 text-neutral-600 dark:text-neutral-300">
                      {TRIGGER_LABELS[execution.triggered_by]}
                    </td>
                    <td className="py-3 px-2 text-neutral-600 dark:text-neutral-300">
                      {formatDateTime(execution.started_at)}
                    </td>
                    <td className="py-3 px-2 text-neutral-600 dark:text-neutral-300 font-mono">
                      {execution.duration_ms > 0 ? formatDurationMs(execution.duration_ms) : '-'}
                    </td>
                    <td className="py-3 px-2 text-neutral-600 dark:text-neutral-300">{execution.attempt}</td>
                  </tr>
                ))}
              </tbody>
            </table>

            {/* Error details for failed executions */}
            {history.some((e) => e.error) && (
              <div className="mt-6 space-y-3">
                <h4 className="font-medium text-neutral-900 dark:text-neutral-100">Error Details</h4>
                {history
                  .filter((e) => e.error)
                  .map((execution) => (
                    <div
                      key={execution.id}
                      className="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
                    >
                      <div className="text-xs text-red-500 dark:text-red-400 mb-1">
                        {formatDateTime(execution.started_at)}
                      </div>
                      <p className="text-sm text-red-700 dark:text-red-300 font-mono break-all">
                        {execution.error}
                      </p>
                    </div>
                  ))}
              </div>
            )}
          </div>
        )}
      </ModalContent>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </ModalFooter>
    </Modal>
  )
}
