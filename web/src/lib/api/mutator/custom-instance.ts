/**
 * Custom Fetch instance for Orval-generated API client
 * Handles base URL, error handling, and request/response transformation
 */

import type { ErrorResponse, CustomInstanceConfig } from './custom-instance.types'

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
 */
export const customInstance = async <T>(config: CustomInstanceConfig): Promise<T> => {
  const { url, params, ...fetchConfig } = config

  // Build URL with query parameters
  let fullUrl = `${API_BASE_URL}${url}`
  if (params) {
    const searchParams = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value))
      }
    })
    const queryString = searchParams.toString()
    if (queryString) {
      fullUrl += `?${queryString}`
    }
  }

  // Make request
  const response = await fetch(fullUrl, {
    ...fetchConfig,
    headers: {
      'Content-Type': 'application/json',
      ...fetchConfig.headers,
    },
  })

  // Handle non-OK responses
  if (!response.ok) {
    let errorData: ErrorResponse | undefined
    try {
      errorData = await response.json()
    } catch {
      // Response might not be JSON
    }

    throw new APIError(
      errorData?.message || errorData?.error || `HTTP ${response.status}`,
      response.status,
      errorData
    )
  }

  // Parse response
  const contentType = response.headers.get('content-type')
  if (contentType?.includes('application/json')) {
    return response.json()
  }

  // For non-JSON responses (like streaming endpoints)
  return response as unknown as T
}
