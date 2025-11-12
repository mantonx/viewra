import type { QueryClient } from '@tanstack/react-query'

// Base API URL - can be overridden by environment variable
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

export interface CustomInstanceOptions extends RequestInit {
  url: string
}

export interface ErrorResponse {
  error: string
  message: string
}

/**
 * Custom fetch instance for API calls
 * This is used by Orval-generated API client
 */
export const customInstance = async <T>({ url, ...config }: CustomInstanceOptions): Promise<T> => {
  const fullUrl = url.startsWith('http') ? url : `${API_BASE_URL}${url}`

  const response = await fetch(fullUrl, {
    ...config,
    headers: {
      'Content-Type': 'application/json',
      ...config.headers,
    },
  })

  if (!response.ok) {
    let errorMessage = 'An error occurred'
    let errorDetail = response.statusText

    try {
      const errorData: ErrorResponse = await response.json()
      errorMessage = errorData.error || errorMessage
      errorDetail = errorData.message || errorDetail
    } catch {
      // If response is not JSON, use status text
    }

    throw new Error(`${errorMessage}: ${errorDetail}`)
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return {} as T
  }

  return response.json()
}

/**
 * Type for custom instance with queryClient
 */
export type CustomInstanceWithQueryClient<T> = (
  config: CustomInstanceOptions,
  queryClient?: QueryClient
) => Promise<T>
