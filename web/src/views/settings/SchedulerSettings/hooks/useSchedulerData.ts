/**
 * Scheduler Data Hook
 * Manages all data fetching and mutations for the scheduler settings view
 */

import { useState, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { schedulerApi } from '@/lib/api'
import { useToast } from '@/lib/hooks/useToast'
import type { TaskStatus, TaskExecution } from '../SchedulerSettings.types'

export interface UseSchedulerDataReturn {
  // Task data
  tasks: TaskStatus[]
  isLoading: boolean
  error: Error | null

  // History
  selectedHistoryTaskId: string | null
  setSelectedHistoryTaskId: (id: string | null) => void
  historyData: TaskExecution[]
  isLoadingHistory: boolean

  // Mutations
  triggerTask: (taskId: string) => void
  enableTask: (taskId: string) => void
  disableTask: (taskId: string) => void
  updateSchedule: (taskId: string, schedule: string) => void

  // Loading states
  isTriggeringTask: (taskId: string) => boolean
  isEnablingTask: (taskId: string) => boolean
  isDisablingTask: (taskId: string) => boolean
  isUpdatingSchedule: boolean
}

export const useSchedulerData = (): UseSchedulerDataReturn => {
  const queryClient = useQueryClient()
  const toast = useToast()

  // Track which tasks are being triggered/enabled/disabled
  const [triggeringTasks, setTriggeringTasks] = useState<Set<string>>(new Set())
  const [enablingTasks, setEnablingTasks] = useState<Set<string>>(new Set())
  const [disablingTasks, setDisablingTasks] = useState<Set<string>>(new Set())

  // Selected task for history viewing
  const [selectedHistoryTaskId, setSelectedHistoryTaskId] = useState<string | null>(null)

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
  const { data: historyData, isLoading: isLoadingHistory } = useQuery({
    queryKey: ['scheduler', 'history', selectedHistoryTaskId],
    queryFn: () => {
      if (!selectedHistoryTaskId) {
        throw new Error('Task ID is required')
      }
      return schedulerApi.getTaskHistory(selectedHistoryTaskId, 20)
    },
    enabled: !!selectedHistoryTaskId,
  })

  // Helper to find task name
  const getTaskName = useCallback(
    (taskId: string) => {
      const task = tasksData?.tasks.find((t) => t.id === taskId)
      return task?.name || taskId
    },
    [tasksData]
  )

  // Trigger task mutation
  const triggerMutation = useMutation({
    mutationFn: (taskId: string) => {
      setTriggeringTasks((prev) => new Set(prev).add(taskId))
      return schedulerApi.triggerTask(taskId)
    },
    onSuccess: (_, taskId) => {
      toast.success(`Task "${getTaskName(taskId)}" triggered successfully`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      toast.error(`Failed to trigger task "${getTaskName(taskId)}": ${error.message}`)
    },
    onSettled: (_, __, taskId) => {
      setTriggeringTasks((prev) => {
        const next = new Set(prev)
        next.delete(taskId)
        return next
      })
    },
  })

  // Enable task mutation
  const enableMutation = useMutation({
    mutationFn: (taskId: string) => {
      setEnablingTasks((prev) => new Set(prev).add(taskId))
      return schedulerApi.enableTask(taskId)
    },
    onSuccess: (_, taskId) => {
      toast.success(`Task "${getTaskName(taskId)}" enabled`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      toast.error(`Failed to enable task "${getTaskName(taskId)}": ${error.message}`)
    },
    onSettled: (_, __, taskId) => {
      setEnablingTasks((prev) => {
        const next = new Set(prev)
        next.delete(taskId)
        return next
      })
    },
  })

  // Disable task mutation
  const disableMutation = useMutation({
    mutationFn: (taskId: string) => {
      setDisablingTasks((prev) => new Set(prev).add(taskId))
      return schedulerApi.disableTask(taskId)
    },
    onSuccess: (_, taskId) => {
      toast.success(`Task "${getTaskName(taskId)}" disabled`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, taskId) => {
      toast.error(`Failed to disable task "${getTaskName(taskId)}": ${error.message}`)
    },
    onSettled: (_, __, taskId) => {
      setDisablingTasks((prev) => {
        const next = new Set(prev)
        next.delete(taskId)
        return next
      })
    },
  })

  // Update schedule mutation
  const updateScheduleMutation = useMutation({
    mutationFn: ({ taskId, schedule }: { taskId: string; schedule: string }) =>
      schedulerApi.updateTaskSchedule(taskId, schedule),
    onSuccess: (_, { taskId }) => {
      toast.success(`Schedule updated for "${getTaskName(taskId)}"`)
      queryClient.invalidateQueries({ queryKey: ['scheduler', 'tasks'] })
    },
    onError: (error: Error, { taskId }) => {
      toast.error(`Failed to update schedule for "${getTaskName(taskId)}": ${error.message}`)
    },
  })

  return {
    // Task data
    tasks: tasksData?.tasks || [],
    isLoading,
    error: error as Error | null,

    // History
    selectedHistoryTaskId,
    setSelectedHistoryTaskId,
    historyData: historyData?.history || [],
    isLoadingHistory,

    // Mutations
    triggerTask: (taskId: string) => triggerMutation.mutate(taskId),
    enableTask: (taskId: string) => enableMutation.mutate(taskId),
    disableTask: (taskId: string) => disableMutation.mutate(taskId),
    updateSchedule: (taskId: string, schedule: string) =>
      updateScheduleMutation.mutate({ taskId, schedule }),

    // Loading states
    isTriggeringTask: (taskId: string) => triggeringTasks.has(taskId),
    isEnablingTask: (taskId: string) => enablingTasks.has(taskId),
    isDisablingTask: (taskId: string) => disablingTasks.has(taskId),
    isUpdatingSchedule: updateScheduleMutation.isPending,
  }
}
