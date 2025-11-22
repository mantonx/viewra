/**
 * Type-safe utilities for extracting data from API responses
 *
 * These utilities handle the common pattern in our API responses where data is wrapped
 * in a structure like { data: { libraries: [...] } }. They provide type-safe extraction
 * with proper fallbacks.
 */

import type {
  GithubComMantonxViewraInternalApplicationLibraryLibraryResponse,
  GithubComMantonxViewraInternalApplicationMediaMediaResponse,
  GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse,
} from '@/lib/api/generated/models'

/**
 * Type guard to check if value is an object
 */
const isObject = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

/**
 * Type guard to check if value is an array
 */
const isArray = (value: unknown): value is unknown[] => {
  return Array.isArray(value)
}

/**
 * Extract libraries array from API response
 *
 * Handles both formats:
 * - Direct: { libraries: [...] }
 * - Nested: { data: { libraries: [...] } }
 *
 * @param response - The API response object
 * @returns Array of library responses, empty array if extraction fails
 */
export const extractLibraries = (
  response: unknown
): GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[] => {
  if (!isObject(response)) {
    return []
  }

  // Check for direct libraries property
  if ('libraries' in response && isArray(response.libraries)) {
    return response.libraries as GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[]
  }

  // Check for nested data.libraries property
  if ('data' in response && isObject(response.data)) {
    if ('libraries' in response.data && isArray(response.data.libraries)) {
      return response.data
        .libraries as GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[]
    }
  }

  return []
}

/**
 * Extract media array from API response
 *
 * Handles both formats:
 * - Direct: { media: [...] }
 * - Nested: { data: { media: [...] } }
 *
 * @param response - The API response object
 * @returns Array of media responses, empty array if extraction fails
 */
export const extractMedia = (
  response: unknown
): GithubComMantonxViewraInternalApplicationMediaMediaResponse[] => {
  if (!isObject(response)) {
    return []
  }

  // Check for direct media property
  if ('media' in response && isArray(response.media)) {
    return response.media as GithubComMantonxViewraInternalApplicationMediaMediaResponse[]
  }

  // Check for nested data.media property
  if ('data' in response && isObject(response.data)) {
    if ('media' in response.data && isArray(response.data.media)) {
      return response.data.media as GithubComMantonxViewraInternalApplicationMediaMediaResponse[]
    }
  }

  return []
}

/**
 * Extract progress array from API response
 *
 * Handles both formats:
 * - Direct: { progress: [...] }
 * - Nested: { data: { progress: [...] } }
 *
 * @param response - The API response object
 * @returns Array of progress responses, empty array if extraction fails
 */
export const extractProgress = (
  response: unknown
): GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse[] => {
  if (!isObject(response)) {
    return []
  }

  // Check for direct progress property
  if ('progress' in response && isArray(response.progress)) {
    return response.progress as GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse[]
  }

  // Check for nested data.progress property
  if ('data' in response && isObject(response.data)) {
    if ('progress' in response.data && isArray(response.data.progress)) {
      return response.data
        .progress as GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse[]
    }
  }

  return []
}

/**
 * Get the default library ID for a specific media type
 *
 * When 'all' is selected, returns the first library of the specified type.
 * This ensures music pages query music libraries, movie pages query movie libraries, etc.
 *
 * @param selectedLibrary - The selected library value ('all' or numeric ID as string)
 * @param libraries - Array of all libraries
 * @param libraryType - The type of library to filter for ('movies', 'tv', 'music')
 * @returns The library ID to use for API queries
 */
export const getLibraryId = (
  selectedLibrary: string,
  libraries: GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[],
  libraryType: 'movies' | 'tv' | 'music'
): number => {
  // If a specific library is selected, use it
  if (selectedLibrary !== 'all') {
    return parseInt(selectedLibrary, 10)
  }

  // For 'all', get the first library of the specified type
  const typedLibraries = libraries.filter(lib => lib.type === libraryType)
  
  // Return the first typed library's ID, or fallback to 1
  return typedLibraries.length > 0 ? typedLibraries[0].id || 1 : 1
}

/**
 * Filter libraries by type
 *
 * Returns only libraries matching the specified media type.
 *
 * @param libraries - Array of all libraries
 * @param libraryType - The type of library to filter for ('movies', 'tv', 'music')
 * @returns Filtered array of libraries
 */
export const filterLibrariesByType = (
  libraries: GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[],
  libraryType: 'movies' | 'tv' | 'music'
): GithubComMantonxViewraInternalApplicationLibraryLibraryResponse[] => {
  return libraries.filter(lib => lib.type === libraryType)
}
