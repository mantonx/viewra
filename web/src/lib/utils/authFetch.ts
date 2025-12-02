/**
 * Authenticated fetch utility for API requests.
 *
 * This module provides a standardized way to make authenticated API requests
 * throughout the frontend. It automatically includes the Authorization header
 * from stored tokens.
 *
 * Usage:
 *   import { authFetch, getAuthHeaders } from '@/lib/utils/authFetch'
 *
 *   // Simple GET request
 *   const response = await authFetch('/api/media/123')
 *
 *   // POST request with body
 *   const response = await authFetch('/api/progress', {
 *     method: 'POST',
 *     body: JSON.stringify({ media_id: 123, progress: 50 })
 *   })
 *
 *   // Just get headers for custom fetch scenarios (e.g., HLS.js)
 *   const headers = getAuthHeaders()
 */

import { API_BASE_URL } from '@/lib/config'

// Storage key for auth tokens (must match AuthContext)
const STORAGE_KEY_TOKENS = 'viewra_auth_tokens'

/**
 * Get the stored access token from localStorage.
 * Returns null if no token is stored or if parsing fails.
 */
export const getStoredAccessToken = (): string | null => {
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

/**
 * Get auth headers object with Authorization bearer token if available.
 * Use this when you need to pass headers to external libraries (e.g., HLS.js).
 */
export const getAuthHeaders = (): Record<string, string> => {
  const headers: Record<string, string> = {}
  const token = getStoredAccessToken()
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  return headers
}

/**
 * Authenticated fetch wrapper that automatically includes auth headers.
 *
 * @param input - URL string or Request object. Relative URLs are prefixed with API_BASE_URL.
 * @param init - Optional RequestInit configuration
 * @returns Promise<Response>
 *
 * Features:
 * - Automatically adds Authorization header if token exists
 * - Prefixes relative URLs with API_BASE_URL
 * - Merges custom headers with auth headers (custom headers take precedence)
 */
export const authFetch = async (
  input: string | URL | Request,
  init?: RequestInit
): Promise<Response> => {
  // Build the full URL
  let url: string
  if (typeof input === 'string') {
    // Prefix relative URLs with API base
    url = input.startsWith('http') ? input : `${API_BASE_URL}${input}`
  } else if (input instanceof URL) {
    url = input.toString()
  } else {
    // Request object - use as-is
    url = input.url
  }

  // Merge auth headers with any provided headers
  const authHeaders = getAuthHeaders()
  const providedHeaders = init?.headers || {}

  // Convert Headers object to plain object if needed
  let headersObj: Record<string, string> = {}
  if (providedHeaders instanceof Headers) {
    providedHeaders.forEach((value, key) => {
      headersObj[key] = value
    })
  } else if (Array.isArray(providedHeaders)) {
    providedHeaders.forEach(([key, value]) => {
      headersObj[key] = value
    })
  } else {
    headersObj = providedHeaders as Record<string, string>
  }

  // Merge headers: auth headers first, then provided headers (so custom can override)
  const mergedHeaders = {
    ...authHeaders,
    ...headersObj,
  }

  return fetch(url, {
    ...init,
    headers: mergedHeaders,
  })
}

/**
 * Authenticated JSON fetch that automatically parses the response.
 *
 * @param input - URL string or Request object
 * @param init - Optional RequestInit configuration
 * @returns Promise<T> - Parsed JSON response
 * @throws Error if response is not ok or JSON parsing fails
 */
export const authFetchJson = async <T = unknown>(
  input: string | URL | Request,
  init?: RequestInit
): Promise<T> => {
  const response = await authFetch(input, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers as Record<string, string> || {}),
    },
  })

  if (!response.ok) {
    const errorText = await response.text().catch(() => response.statusText)
    throw new Error(`Request failed: ${response.status} ${errorText}`)
  }

  return response.json()
}
