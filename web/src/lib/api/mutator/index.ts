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

// Storage keys for auth tokens (must match AuthContext)
const STORAGE_KEY_TOKENS = 'viewra_auth_tokens'
const STORAGE_KEY_USER = 'viewra_auth_user'

// API base URL
const API_BASE = ''

// Token refresh buffer (refresh 1 minute before expiry)
const REFRESH_BUFFER_MS = 60 * 1000

// Track if we're currently refreshing to prevent multiple concurrent refreshes
let isRefreshing = false
let refreshPromise: Promise<boolean> | null = null

// Helper to get access token from storage
const getStoredAccessToken = (): string | null => {
  try {
    const tokensStr = localStorage.getItem(STORAGE_KEY_TOKENS)
    if (!tokensStr) {
      return null
    }
    const tokens = JSON.parse(tokensStr)
    return tokens.accessToken || null
  } catch {
    return null
  }
}

// Helper to get refresh token from storage
const getStoredRefreshToken = (): string | null => {
  try {
    const tokensStr = localStorage.getItem(STORAGE_KEY_TOKENS)
    if (!tokensStr) {
      return null
    }
    const tokens = JSON.parse(tokensStr)
    return tokens.refreshToken || null
  } catch {
    return null
  }
}

// Helper to check if token is about to expire
const isTokenExpiringSoon = (): boolean => {
  try {
    const tokensStr = localStorage.getItem(STORAGE_KEY_TOKENS)
    if (!tokensStr) {
      return true
    }
    const tokens = JSON.parse(tokensStr)
    return (tokens.expiresAt - Date.now()) < REFRESH_BUFFER_MS
  } catch {
    return true
  }
}

// Helper to save new tokens
const saveTokens = (accessToken: string, refreshToken: string, expiresIn: number, user: unknown) => {
  const tokens = {
    accessToken,
    refreshToken,
    expiresAt: Date.now() + expiresIn * 1000,
  }
  localStorage.setItem(STORAGE_KEY_TOKENS, JSON.stringify(tokens))
  localStorage.setItem(STORAGE_KEY_USER, JSON.stringify(user))
}

// Helper to clear auth data
const clearAuth = () => {
  localStorage.removeItem(STORAGE_KEY_TOKENS)
  localStorage.removeItem(STORAGE_KEY_USER)
}

// Attempt to refresh the access token
const refreshAccessToken = async (): Promise<boolean> => {
  // If already refreshing, wait for that to complete
  if (isRefreshing && refreshPromise) {
    return refreshPromise
  }

  const refreshToken = getStoredRefreshToken()
  if (!refreshToken) {
    return false
  }

  isRefreshing = true
  refreshPromise = (async () => {
    try {
      const response = await fetch(`${API_BASE}/api/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })

      if (!response.ok) {
        clearAuth()
        return false
      }

      const data = await response.json()
      saveTokens(
        data.AccessToken,
        data.RefreshToken,
        data.ExpiresIn,
        data.User
      )
      return true
    } catch {
      clearAuth()
      return false
    } finally {
      isRefreshing = false
      refreshPromise = null
    }
  })()

  return refreshPromise
}

// Dispatch custom event to notify AuthContext of logout
const dispatchLogoutEvent = () => {
  window.dispatchEvent(new CustomEvent('auth:logout'))
}

/**
 * Custom fetch instance for API calls
 * This is used by Orval-generated API client
 * Supports both signatures: customInstance(url, options) and customInstance({ url, ...options })
 *
 * Handles 401 responses by attempting token refresh and retrying the request.
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

  // Build headers with auth token if available
  const buildHeaders = (): Record<string, string> => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((fetchConfig.headers as Record<string, string>) || {}),
    }

    // Add Authorization header if token exists and not already set
    const accessToken = getStoredAccessToken()
    if (accessToken && !headers['Authorization']) {
      headers['Authorization'] = `Bearer ${accessToken}`
    }

    return headers
  }

  // Perform the actual request
  const performRequest = async (): Promise<Response> => {
    const requestInit: RequestInit = {
      ...fetchConfig,
      headers: buildHeaders(),
    }

    // Add body if data is provided
    if (data !== undefined) {
      requestInit.body = JSON.stringify(data)
    }

    return fetch(fullUrl, requestInit)
  }

  // Proactively refresh if token is expiring soon (before making the request)
  if (isTokenExpiringSoon() && getStoredRefreshToken()) {
    await refreshAccessToken()
  }

  let response = await performRequest()

  // Handle 401 Unauthorized - attempt token refresh and retry
  if (response.status === 401) {
    // Skip refresh for auth endpoints to avoid infinite loops
    const isAuthEndpoint = url.includes('/api/auth/')

    if (!isAuthEndpoint) {
      const refreshSuccess = await refreshAccessToken()

      if (refreshSuccess) {
        // Retry the request with new token
        response = await performRequest()
      } else {
        // Refresh failed - clear auth and notify
        clearAuth()
        dispatchLogoutEvent()
      }
    }
  }

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

    // For 401 errors after refresh attempt, include helpful message
    if (response.status === 401) {
      throw new Error('Session expired. Please log in again.')
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
