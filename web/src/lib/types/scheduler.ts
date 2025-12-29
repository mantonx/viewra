/**
 * Scheduler Types
 */

export type TaskSource = 'internal' | 'plugin'

export type ExecutionStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'skipped'
  | 'interrupted'

export type TriggeredBy = 'schedule' | 'manual' | 'retry' | 'dependency'

export interface TaskStatus {
  id: string
  name: string
  description: string
  schedule: string
  enabled: boolean
  source: TaskSource
  source_id?: string
  is_running: boolean
  last_run?: string
  next_run?: string
  last_success?: boolean
  last_error?: string
}

export interface TaskExecution {
  id: string
  task_id: string
  status: ExecutionStatus
  triggered_by: TriggeredBy
  started_at?: string
  ended_at?: string
  duration_ms: number
  success?: boolean
  error?: string
  attempt: number
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
  execution_id: string
}

export interface TaskOperationResponse {
  message: string
  task_id: string
}

export interface RunningExecutionsResponse {
  executions: TaskExecution[]
  count: number
}

export interface CancelExecutionResponse {
  message: string
  execution_id: string
}
