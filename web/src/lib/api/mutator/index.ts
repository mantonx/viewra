import type { QueryClient } from '@tanstack/react-query'
import { API_BASE_URL } from '@/lib/config'

export interface CustomInstanceOptions extends RequestInit {
  url?: string
  data?: unknown
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
  let config: RequestInit & { data?: unknown }

  if (typeof urlOrConfig === 'string') {
    // Called as customInstance(url, options)
    url = urlOrConfig
    config = maybeConfig || {}
  } else {
    // Called as customInstance({ url, ...options })
    const { url: extractedUrl, data, ...rest } = urlOrConfig
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    url = extractedUrl!
    config = { ...rest, data }
  }

  const fullUrl = url.startsWith('http') ? url : `${API_BASE_URL}${url}`

  // Extract data and convert to body
  const { data, ...fetchConfig } = config
  const requestInit: RequestInit = {
    ...fetchConfig,
    headers: {
      'Content-Type': 'application/json',
      ...fetchConfig.headers,
    },
  }

  // Add body if data is provided
  if (data !== undefined) {
    requestInit.body = JSON.stringify(data)
  }

  const response = await fetch(fullUrl, requestInit)

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
    return {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      data: {} as any,
      status: response.status,
      headers: response.headers,
    } as T
  }

  const responseData = await response.json()

  // Return the structure expected by Orval-generated code
  return {
    data: responseData,
    status: response.status,
    headers: response.headers,
  } as T
}

/**
 * Type for custom instance with queryClient
 */
export type CustomInstanceWithQueryClient<T> = (
  config: CustomInstanceOptions,
  queryClient?: QueryClient
) => Promise<T>
