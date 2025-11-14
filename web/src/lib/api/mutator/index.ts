import type { QueryClient } from '@tanstack/react-query'
import { API_BASE_URL } from '@/lib/config'

export interface CustomInstanceOptions extends RequestInit {
  url?: string
}

export interface ErrorResponse {
  error: string
  message: string
}

/**
 * Custom fetch instance for API calls
 * This is used by Orval-generated API client
 * Supports both signatures: customInstance(url, options) and customInstance({ url, ...options })
 */
export const customInstance = async <T>(
  urlOrConfig: string | CustomInstanceOptions,
  maybeConfig?: RequestInit
): Promise<T> => {
  // Handle both call signatures
  let url: string
  let config: RequestInit

  if (typeof urlOrConfig === 'string') {
    // Called as customInstance(url, options)
    url = urlOrConfig
    config = maybeConfig || {}
  } else {
    // Called as customInstance({ url, ...options })
    const { url: extractedUrl, ...rest } = urlOrConfig
    url = extractedUrl!
    config = rest
  }

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
