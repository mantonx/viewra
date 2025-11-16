/**
 * Scheduler API Client
 */

import { customInstance } from './mutator/index'
import type {
  TaskListResponse,
  TaskStatus,
  TaskHistoryResponse,
  TriggerTaskResponse,
  TaskOperationResponse,
} from '../types/scheduler'

const BASE_URL = '/api/admin/scheduler'

export const schedulerApi = {
  /**
   * List all scheduled tasks
   */
  listTasks: async (): Promise<TaskListResponse> => {
    const response = await customInstance<TaskListResponse>({
      url: `${BASE_URL}/tasks`,
      method: 'GET',
    })
    return response
  },

  /**
   * Get status of a specific task
   */
  getTaskStatus: async (taskId: string): Promise<TaskStatus> => {
    const response = await customInstance<TaskStatus>({
      url: `${BASE_URL}/tasks/${taskId}`,
      method: 'GET',
    })
    return response
  },

  /**
   * Manually trigger a task execution
   */
  triggerTask: async (taskId: string): Promise<TriggerTaskResponse> => {
    const response = await customInstance<TriggerTaskResponse>({
      url: `${BASE_URL}/tasks/${taskId}/trigger`,
      method: 'POST',
    })
    return response
  },

  /**
   * Get execution history for a task
   */
  getTaskHistory: async (
    taskId: string,
    limit: number = 10
  ): Promise<TaskHistoryResponse> => {
    const response = await customInstance<TaskHistoryResponse>({
      url: `${BASE_URL}/tasks/${taskId}/history?limit=${limit}`,
      method: 'GET',
    })
    return response
  },

  /**
   * Enable a disabled task
   */
  enableTask: async (taskId: string): Promise<TaskOperationResponse> => {
    const response = await customInstance<TaskOperationResponse>({
      url: `${BASE_URL}/tasks/${taskId}/enable`,
      method: 'POST',
    })
    return response
  },

  /**
   * Disable an enabled task
   */
  disableTask: async (taskId: string): Promise<TaskOperationResponse> => {
    const response = await customInstance<TaskOperationResponse>({
      url: `${BASE_URL}/tasks/${taskId}/disable`,
      method: 'POST',
    })
    return response
  },

  /**
   * Update task schedule
   */
  updateTaskSchedule: async (
    taskId: string,
    schedule: string
  ): Promise<TaskOperationResponse> => {
    const response = await customInstance<TaskOperationResponse>({
      url: `${BASE_URL}/tasks/${taskId}/schedule`,
      method: 'PUT',
      data: { schedule },
    })
    return response
  },
}
