/**
 * Hook for executing streaming actions with SSE progress tracking.
 *
 * Provides a convenient way to execute plugin actions that return SSE streams
 * with progress updates (like model downloads).
 *
 * @example
 * ```tsx
 * const { execute, progress, isStreaming, cancel } = useStreamingAction('ollama', {
 *   onComplete: () => refetch(),
 * })
 *
 * // Execute streaming action
 * await execute('/models/pull', { model: 'llama3' })
 * ```
 */

import { useState, useCallback, useRef, useEffect } from 'react'
import { pluginApi } from '@/lib/api/pluginApi'
import type {
  StreamingProgress,
  UseStreamingActionOptions,
  UseStreamingActionReturn,
} from './useStreamingAction.types'

/**
 * Parse SSE events from a ReadableStream
 */
const parseSSEStream = async function* <T>(
  reader: ReadableStreamDefaultReader<Uint8Array>
): AsyncGenerator<T> {
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          const data = JSON.parse(line.slice(6)) as T
          yield data
        } catch {
          // Ignore parse errors for incomplete JSON
        }
      }
    }
  }
}

export const useStreamingAction = (
  pluginId: string,
  options: UseStreamingActionOptions = {}
): UseStreamingActionReturn => {
  const { onProgress, onComplete, onError } = options

  const [progress, setProgress] = useState<StreamingProgress | null>(null)
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const abortRef = useRef<(() => void) | null>(null)
  const readerRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null)

  // Store callbacks in refs to avoid stale closures
  const onProgressRef = useRef(onProgress)
  const onCompleteRef = useRef(onComplete)
  const onErrorRef = useRef(onError)

  useEffect(() => {
    onProgressRef.current = onProgress
    onCompleteRef.current = onComplete
    onErrorRef.current = onError
  }, [onProgress, onComplete, onError])

  const cancel = useCallback(() => {
    abortRef.current?.()
    readerRef.current?.cancel()
    setIsStreaming(false)
    setProgress(null)
  }, [])

  const execute = useCallback(
    async (endpoint: string, data?: unknown): Promise<void> => {
      // Cancel any existing operation
      cancel()

      setIsStreaming(true)
      setError(null)
      setProgress({ status: 'Starting...' })

      try {
        const { response, abort } = await pluginApi.stream(pluginId, endpoint, data)
        abortRef.current = abort

        if (!response.body) {
          throw new Error('No response body')
        }

        const reader = response.body.getReader()
        readerRef.current = reader

        for await (const event of parseSSEStream<StreamingProgress>(reader)) {
          setProgress(event)
          onProgressRef.current?.(event)

          if (event.error) {
            const err = new Error(event.error)
            setError(err)
            onErrorRef.current?.(err)
            break
          }

          if (event.done) {
            onCompleteRef.current?.()
          }
        }
      } catch (err) {
        if ((err as Error).name !== 'AbortError') {
          const error = err instanceof Error ? err : new Error(String(err))
          setError(error)
          onErrorRef.current?.(error)
        }
      } finally {
        readerRef.current = null
        abortRef.current = null
        setIsStreaming(false)
      }
    },
    [pluginId, cancel]
  )

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      abortRef.current?.()
      readerRef.current?.cancel()
    }
  }, [])

  return {
    execute,
    progress,
    isStreaming,
    cancel,
    error,
  }
}

export type { StreamingProgress, UseStreamingActionOptions, UseStreamingActionReturn }
