/**
 * Scheduler Types
 */

export interface TaskStatus {
  id: string
  name: string
  description: string
  schedule: string
  enabled: boolean
  is_running: boolean
  last_run?: string
  next_run?: string
  last_success?: boolean
  last_error?: string
}

export interface TaskExecution {
  task_id: string
  started_at: string
  ended_at: string
  duration_ms: number
  success: boolean
  error?: string
}

export interface TaskHistoryResponse {
  task_id: string
  history: TaskExecution[]
  count: number
}

export interface TaskListResponse {
  tasks: TaskStatus[]
  count: number
}

export interface TriggerTaskResponse {
  message: string
  task_id: string
}

export interface TaskOperationResponse {
  message: string
  task_id: string
}
