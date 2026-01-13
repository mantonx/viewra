/**
 * Search Hooks
 *
 * React hooks for using the search abstraction.
 * These hooks are plugin-agnostic - they work with any registered provider.
 */

import { useQuery } from '@tanstack/react-query'
import { getSearchProvider } from './registry'
import { builtinSearchProvider } from './builtin'
import type { SearchOptions, SearchResult } from './types'

export interface UseSearchOptions extends SearchOptions {
  /** Whether to enable the query */
  enabled?: boolean

  /** Minimum query length before searching */
  minLength?: number
}

/**
 * Hook to perform search using the best available provider
 *
 * This hook automatically uses enhanced search if a plugin is available,
 * or falls back to built-in search otherwise.
 *
 * @example
 * ```typescript
 * const { data, isLoading, error } = useSearch('action movies', {
 *   entityTypes: ['movie'],
 *   limit: 50
 * })
 *
 * // data.items - the search results
 * // data.enhancement - optional UI from plugin (e.g., intent chips)
 * ```
 */
export function useSearch<T = unknown>(
  query: string,
  options?: UseSearchOptions
) {
  const minLength = options?.minLength ?? 2

  return useQuery<SearchResult<T>>({
    queryKey: ['search', query, options],
    queryFn: async () => {
      // Get best available provider
      const provider = getSearchProvider()

      // Use provider or fallback to built-in
      const searchProvider = provider || builtinSearchProvider

      return searchProvider.search(query, options) as Promise<SearchResult<T>>
    },
    enabled: options?.enabled !== false && query.length >= minLength,
    staleTime: 30 * 1000, // Cache for 30 seconds
  })
}

/**
 * Check if an enhanced search provider is available
 *
 * Returns true if a plugin provides search, false if only built-in is available
 */
export function useHasEnhancedSearch(): boolean {
  const provider = getSearchProvider()

  // Enhanced search available if we have a provider that's not the built-in
  return provider !== null && provider !== builtinSearchProvider
}
