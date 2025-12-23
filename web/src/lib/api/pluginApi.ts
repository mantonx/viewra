/**
 * Plugin API utility for making requests to plugin endpoints.
 *
 * Plugins expose dynamic endpoints at /api/plugin/{pluginId}/{endpoint}.
 * This utility provides a standardized way to interact with these endpoints.
 */

import { authFetch } from '@/lib/utils/authFetch'

export type PluginApiError = {
  status: number
  error: string
  message?: string
}

/**
 * Parse error response from plugin API
 */
const parseError = async (response: Response): Promise<PluginApiError> => {
  try {
    const data = await response.json()
    return {
      status: response.status,
      error: data.error || `HTTP ${response.status}`,
      message: data.message,
    }
  } catch {
    return {
      status: response.status,
      error: `HTTP ${response.status}`,
    }
  }
}

/**
 * Build the full plugin endpoint URL
 */
const buildUrl = (pluginId: string, endpoint: string): string => {
  const normalizedEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`
  return `/api/plugin/${pluginId}${normalizedEndpoint}`
}

/**
 * GET request to a plugin endpoint
 */
const get = async <T = unknown>(pluginId: string, endpoint: string): Promise<T> => {
  const response = await authFetch(buildUrl(pluginId, endpoint), {
    method: 'GET',
    credentials: 'include',
  })

  if (!response.ok) {
    const error = await parseError(response)
    throw new Error(error.message || error.error)
  }

  return response.json()
}

/**
 * POST request to a plugin endpoint
 */
const post = async <T = unknown>(
  pluginId: string,
  endpoint: string,
  data?: unknown
): Promise<T> => {
  const response = await authFetch(buildUrl(pluginId, endpoint), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: data ? JSON.stringify(data) : undefined,
  })

  if (!response.ok) {
    const error = await parseError(response)
    throw new Error(error.message || error.error)
  }

  return response.json()
}

/**
 * PUT request to a plugin endpoint
 */
const put = async <T = unknown>(
  pluginId: string,
  endpoint: string,
  data?: unknown
): Promise<T> => {
  const response = await authFetch(buildUrl(pluginId, endpoint), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: data ? JSON.stringify(data) : undefined,
  })

  if (!response.ok) {
    const error = await parseError(response)
    throw new Error(error.message || error.error)
  }

  return response.json()
}

/**
 * DELETE request to a plugin endpoint
 */
const del = async (pluginId: string, endpoint: string): Promise<void> => {
  const response = await authFetch(buildUrl(pluginId, endpoint), {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    const error = await parseError(response)
    throw new Error(error.message || error.error)
  }
}

export type StreamingResponse = {
  response: Response
  abort: () => void
}

/**
 * POST request that returns a streaming response (for SSE endpoints)
 */
const stream = async (
  pluginId: string,
  endpoint: string,
  data?: unknown
): Promise<StreamingResponse> => {
  const abortController = new AbortController()

  const response = await authFetch(buildUrl(pluginId, endpoint), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: data ? JSON.stringify(data) : undefined,
    signal: abortController.signal,
  })

  if (!response.ok) {
    const error = await parseError(response)
    throw new Error(error.message || error.error)
  }

  return {
    response,
    abort: () => abortController.abort(),
  }
}

/**
 * Plugin API client for making requests to plugin endpoints.
 *
 * @example
 * ```ts
 * // GET request
 * const data = await pluginApi.get<ModelList>('ollama', '/models')
 *
 * // POST request
 * const result = await pluginApi.post('ollama', '/models/pull', { model: 'llama3' })
 *
 * // Streaming request
 * const { response, abort } = await pluginApi.stream('ollama', '/models/pull', { model: 'llama3' })
 * ```
 */
export const pluginApi = {
  get,
  post,
  put,
  delete: del,
  stream,
}
