/**
 * API utility functions for handling API responses
 */

/**
 * Safely extracts data from an API response object.
 * Handles the nested structure: response.data[key]
 *
 * @param response - The API response object
 * @param key - The key to extract from response.data
 * @returns Array of extracted data, or empty array if not found
 *
 * @example
 * const libraries = extractApiData(apiResponse, 'libraries')
 * const media = extractApiData(apiResponse, 'media')
 */
export function extractApiData<T>(response: unknown, key: string): T[] {
  if (!response || typeof response !== 'object') {
    return []
  }

  if ('data' in response && response.data && typeof response.data === 'object') {
    if (key in response.data) {
      const value = (response.data as Record<string, unknown>)[key]
      return Array.isArray(value) ? (value as T[]) : []
    }
  }

  return []
}
