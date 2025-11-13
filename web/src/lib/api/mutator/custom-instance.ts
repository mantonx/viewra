/**
 * Custom Fetch instance for Orval-generated API client
 * Handles base URL, error handling, and request/response transformation
 */

import type { ErrorResponse } from './custom-instance.types'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export class APIError extends Error {
  status: number
  response?: ErrorResponse

  constructor(message: string, status: number, response?: ErrorResponse) {
    super(message)
    this.name = 'APIError'
    this.status = status
    this.response = response
  }
}

/**
 * Custom fetch instance with error handling
 * This signature matches orval's default generated code: (url: string, options: RequestInit)
 */
export const customInstance = async <T>(url: string, options?: RequestInit): Promise<T> => {
  // Build full URL
  const fullUrl = `${API_BASE_URL}${url}`

  // Make request
  const response = await fetch(fullUrl, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  // Parse response data
  const contentType = response.headers.get('content-type')
  let data: unknown

  if (contentType?.includes('application/json')) {
    data = await response.json()
  } else {
    // For non-JSON responses (like streaming endpoints)
    data = response
  }

  // Handle non-OK responses
  if (!response.ok) {
    const errorData = data as ErrorResponse | undefined
    throw new APIError(
      errorData?.message || errorData?.error || `HTTP ${response.status}`,
      response.status,
      errorData
    )
  }

  // Return response in orval format: { data, status, headers }
  return {
    data,
    status: response.status,
    headers: response.headers,
  } as T
}
