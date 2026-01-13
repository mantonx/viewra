/**
 * Built-in Fallback Search Provider
 *
 * Basic text search used when no plugin provides search capability.
 * This ensures the app always has working search.
 */

import type { SearchProvider, SearchOptions, SearchResult } from './types'
import { moviesApi } from '@/lib/api/movies'
import type { Movie } from '@/lib/types/movies'

/**
 * Built-in search provider using the core /api/movies endpoint
 */
class BuiltinSearchProvider implements SearchProvider<Movie> {
  async search(query: string, options?: SearchOptions): Promise<SearchResult<Movie>> {
    try {
      // Use existing movies API search
      const response = await moviesApi.search({
        search: query,
        limit: options?.limit || 100,
      })

      if (response.status === 200 && response.data) {
        return {
          items: response.data,
          total: response.data.length,
          fallback: true, // Indicate this is fallback search
        }
      }

      return {
        items: [],
        total: 0,
        fallback: true,
      }
    } catch (error) {
      console.error('Built-in search failed:', error)
      return {
        items: [],
        total: 0,
        fallback: true,
      }
    }
  }

  isAvailable(): boolean {
    // Built-in search is always available
    return true
  }

  priority = 0 // Lowest priority - only used as fallback
}

export const builtinSearchProvider = new BuiltinSearchProvider()
