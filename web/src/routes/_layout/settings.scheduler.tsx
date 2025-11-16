import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { schedulerApi } from '@/lib/api'
import { Button, Card, CardHeader, CardContent } from '@/components/ui'
import { PageHeader, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { ScheduleEditor } from '@/components/scheduler'
import { cronToReadable } from '@/lib/utils/cron'
import type { TaskStatus, TaskExecution } from '@/lib/types/scheduler'

const SchedulerSettings = () => {
  const queryClient = useQueryClient()
  const toast = useToast()
  const [selectedTask, setSelectedTask] = useState<string | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [editingSchedule, setEditingSchedule] = useState<string | null>(null)

  // Fetch all tasks
  const {
    data: tasksData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['scheduler', 'tasks'],
    queryFn: () => schedulerApi.listTasks(),
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  // Fetch task history when a task is selected
  const { data: historyData } = useQuery({
    queryKey: ['scheduler', 'history', selectedTask],
    queryFn: () => {
      if (!selectedTask) {
        throw new Error('Task ID is required')
      }
      return schedulerApi.getTaskHistory(selectedTask, 20)
    },
    enabled: !!selectedTask && showHistory,
  })

  // Trigger task mutation
  const triggerMutation = useMutation({
    mutationFn: (taskId: string) => schedulerApi.triggerTask(taskId),
    onSuccess: (_, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.success(`Task "${task?.name || taskId}" triggered successfully`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.error(`Failed to trigger task "${task?.name || taskId}": ${error.message}`)
    },
  })

  // Enable task mutation
  const enableMutation = useMutation({
    mutationFn: (taskId: string) => schedulerApi.enableTask(taskId),
    onSuccess: (_, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.success(`Task "${task?.name || taskId}" enabled`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.error(`Failed to enable task "${task?.name || taskId}": ${error.message}`)
    },
  })

  // Disable task mutation
  const disableMutation = useMutation({
    mutationFn: (taskId: string) => schedulerApi.disableTask(taskId),
    onSuccess: (_, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.success(`Task "${task?.name || taskId}" disabled`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.error(`Failed to disable task "${task?.name || taskId}": ${error.message}`)
    },
  })

  // Update schedule mutation
  const updateScheduleMutation = useMutation({
    mutationFn: ({ taskId, schedule }: { taskId: string; schedule: string }) =>
      schedulerApi.updateTaskSchedule(taskId, schedule),
    onSuccess: (_, { taskId, schedule }) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      const readableSchedule = cronToReadable(schedule)
      toast.success(`Schedule updated for "${task?.name || taskId}" to: ${readableSchedule}`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
      setEditingSchedule(null)
    },
    onError: (error: Error, { taskId }) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      toast.error(`Failed to update schedule for "${task?.name || taskId}": ${error.message}`)
    },
  })

  if (isLoading) {
    return <LoadingPage text="Loading scheduler tasks..." />
  }

  if (error) {
    return <ErrorPage error={error} context="scheduler" />
  }

  const tasks = tasksData?.tasks || []

  const formatDate = (dateStr?: string) => {
    if (!dateStr || dateStr === '0001-01-01T00:00:00Z') {return 'Never'}
    return new Date(dateStr).toLocaleString()
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) {return `${ms}ms`}
    if (ms < 60000) {return `${(ms / 1000).toFixed(1)}s`}
    return `${(ms / 60000).toFixed(1)}m`
  }

  const TaskCard = ({ task }: { task: TaskStatus }) => (
    <div
      className={`
        p-4 border-b last:border-b-0 hover:bg-gray-50 transition-all duration-200
        ${!task.enabled ? 'bg-gray-50/50' : ''}
      `}
    >
      <div className="flex items-start justify-between">
        <div
          className={`flex-1 ${!task.enabled ? 'opacity-50' : ''}`}
        >
          <div className="flex items-center gap-2">
            <h3
              className={`text-lg font-semibold ${
                !task.enabled ? 'text-gray-500' : ''
              }`}
            >
              {task.name}
            </h3>
            {task.is_running && (
              <span className="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded">
                Running
              </span>
            )}
            {!task.enabled && (
              <span className="px-2 py-1 text-xs bg-gray-200 text-gray-500 rounded">
                Disabled
              </span>
            )}
            {task.last_success === false && task.last_error && (
              <span className="px-2 py-1 text-xs bg-red-100 text-red-800 rounded">
                Last run failed
              </span>
            )}
          </div>
          <p
            className={`text-sm mt-1 ${
              !task.enabled ? 'text-gray-400' : 'text-gray-600'
            }`}
          >
            {task.description}
          </p>
          <div className="grid grid-cols-2 gap-4 mt-3 text-sm">
            <div className="col-span-2">
              <span className="text-gray-500">Schedule:</span>
              <span
                className={`ml-2 font-medium ${
                  !task.enabled ? 'text-gray-400' : ''
                }`}
              >
                {cronToReadable(task.schedule)}
              </span>
              <span className="ml-2 text-xs text-gray-400 font-mono">
                ({task.schedule})
              </span>
            </div>
            <div>
              <span className="text-gray-500">Next run:</span>
              <span className={`ml-2 ${!task.enabled ? 'text-gray-400' : ''}`}>
                {!task.enabled ? 'N/A' : formatDate(task.next_run)}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Last run:</span>
              <span className={`ml-2 ${!task.enabled ? 'text-gray-400' : ''}`}>
                {formatDate(task.last_run)}
              </span>
            </div>
            {task.last_error && (
              <div className="col-span-2">
                <span className="text-gray-500">Last error:</span>
                <span className="ml-2 text-red-600 font-mono text-xs">
                  {task.last_error}
                </span>
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-2 ml-4">
          {task.enabled ? (
            <>
              <Button
                size="sm"
                onClick={() => triggerMutation.mutate(task.id)}
                disabled={task.is_running || triggerMutation.isPending}
              >
                {triggerMutation.isPending ? 'Triggering...' : 'Run Now'}
              </Button>

              <Button
                size="sm"
                variant="secondary"
                onClick={() => setEditingSchedule(task.id)}
              >
                Edit Schedule
              </Button>

              <Button
                size="sm"
                variant="secondary"
                onClick={() => {
                  setSelectedTask(task.id)
                  setShowHistory(true)
                }}
              >
                View History
              </Button>

              <Button
                size="sm"
                variant="secondary"
                onClick={() => disableMutation.mutate(task.id)}
                disabled={disableMutation.isPending}
              >
                Disable
              </Button>
            </>
          ) : (
            <>
              <Button
                size="sm"
                onClick={() => enableMutation.mutate(task.id)}
                disabled={enableMutation.isPending}
                className="bg-green-600 hover:bg-green-700 text-white"
              >
                {enableMutation.isPending ? 'Enabling...' : 'Enable Task'}
              </Button>

              <Button
                size="sm"
                variant="secondary"
                onClick={() => {
                  setSelectedTask(task.id)
                  setShowHistory(true)
                }}
              >
                View History
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )

  const HistoryModal = () => {
    if (!showHistory || !selectedTask) {return null}

    const task = tasks.find((t) => t.id === selectedTask)
    const executions = historyData?.history || []

    return (
      <div
        className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
        onClick={() => setShowHistory(false)}
      >
        <div
          className="bg-white rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[80vh] overflow-hidden"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="p-6 border-b">
            <h2 className="text-xl font-semibold">
              Execution History: {task?.name}
            </h2>
            <button
              onClick={() => setShowHistory(false)}
              className="absolute top-6 right-6 text-gray-400 hover:text-gray-600"
            >
              ✕
            </button>
          </div>

          <div className="overflow-y-auto max-h-[60vh]">
            {executions.length === 0 ? (
              <div className="p-8 text-center text-gray-500">
                No execution history yet
              </div>
            ) : (
              <table className="w-full">
                <thead className="bg-gray-50 sticky top-0">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Started
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Duration
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Error
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {executions.map((exec: TaskExecution, idx: number) => (
                    <tr key={idx} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {formatDate(exec.started_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {formatDuration(exec.duration_ms)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {exec.success ? (
                          <span className="px-2 py-1 text-xs bg-green-100 text-green-800 rounded">
                            Success
                          </span>
                        ) : (
                          <span className="px-2 py-1 text-xs bg-red-100 text-red-800 rounded">
                            Failed
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-500 font-mono text-xs">
                        {exec.error || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="p-4 border-t bg-gray-50">
            <Button onClick={() => setShowHistory(false)} variant="secondary">
              Close
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <PageHeader
        title="Scheduler"
        description="View and manage scheduled tasks. Tasks run automatically on their configured schedule."
      />

      <Card className="mt-6">
        <CardHeader>
          <h2 className="text-lg font-semibold">
            Scheduled Tasks ({tasks.length})
          </h2>
        </CardHeader>

        {tasks.length === 0 ? (
          <CardContent>
            <EmptyState
              icon="⏰"
              title="No scheduled tasks"
              description="No tasks are currently registered with the scheduler."
            />
          </CardContent>
        ) : (
          <div className="divide-y">
            {tasks.map((task) => (
              <TaskCard key={task.id} task={task} />
            ))}
          </div>
        )}
      </Card>

      <HistoryModal />

      {/* Schedule Editor Modal */}
      {editingSchedule && (() => {
        const task = tasks.find((t) => t.id === editingSchedule)
        if (!task) {return null}

        return (
          <ScheduleEditor
            taskId={task.id}
            taskName={task.name}
            currentSchedule={task.schedule}
            onSave={(schedule) => {
              updateScheduleMutation.mutate({ taskId: task.id, schedule })
            }}
            onCancel={() => setEditingSchedule(null)}
            isSubmitting={updateScheduleMutation.isPending}
          />
        )
      })()}
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/scheduler')({
  component: SchedulerSettings,
})
