/**
 * Types for useStreamingAction hook.
 */

/** Progress data from SSE streaming action */
export type StreamingProgress = {
  /** Current status message */
  status?: string
  /** Total items/bytes to process */
  total?: number
  /** Completed items/bytes */
  completed?: number
  /** Percentage complete (0-100) */
  percent?: number
  /** Whether the operation is complete */
  done?: boolean
  /** Error message if failed */
  error?: string
}

/** Options for useStreamingAction hook */
export type UseStreamingActionOptions = {
  /** Called when progress updates */
  onProgress?: (progress: StreamingProgress) => void
  /** Called when operation completes successfully */
  onComplete?: () => void
  /** Called when an error occurs */
  onError?: (error: Error) => void
}

/** Return value from useStreamingAction hook */
export type UseStreamingActionReturn = {
  /** Execute a streaming action */
  execute: (endpoint: string, data?: unknown) => Promise<void>
  /** Current progress state */
  progress: StreamingProgress | null
  /** Whether a streaming action is in progress */
  isStreaming: boolean
  /** Cancel the current streaming action */
  cancel: () => void
  /** Error from the last action */
  error: Error | null
}
