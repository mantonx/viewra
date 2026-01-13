/**
 * Semantic Search Provider
 *
 * Implements the SearchProvider interface to provide enhanced search.
 * This file bridges the plugin-specific code with the core abstraction.
 */

import type { SearchProvider, SearchOptions, SearchResult } from '@/lib/search'
import { semanticSearchApi } from './api/client'
import { IntentChipsBar } from './components/IntentChipsBar'
import { moviesApi } from '@/lib/api/movies'
import type { Movie } from '@/lib/types/movies'
import { logger } from '@/lib/utils/logger'

class SemanticSearchProvider implements SearchProvider<Movie> {
  private available: boolean | null = null

  async search(query: string, options?: SearchOptions): Promise<SearchResult<Movie>> {
    try {
      // Call semantic search API
      const response = await semanticSearchApi.search({
        query,
        entity_types: options?.entityTypes || ['movie'],
        limit: options?.limit || 100,
      })

      // Hydrate entity IDs to full Movie objects
      const movies = await this.hydrateMovies(response.results.map((r) => r.entity_id))

      // Build enhancement UI with intent chips
      const enhancement = response.intent_chips && response.intent_chips.length > 0 ? (
        <IntentChipsBar
          chips={response.intent_chips}
          onRemoveChip={(chipId) => {
            // TODO: Implement smart chip removal
            logger.debug('Remove chip:', chipId)
          }}
        />
      ) : undefined

      return {
        items: movies,
        total: response.total,
        enhancement,
        fallback: false,
      }
    } catch (error) {
      logger.error('Semantic search failed:', error)

      // Return empty results on error
      return {
        items: [],
        total: 0,
        fallback: true,
      }
    }
  }

  isAvailable(): boolean {
    // Cache availability check
    if (this.available !== null) {
      return this.available
    }

    // Default to true, will be checked async
    return true
  }

  /**
   * Check availability asynchronously
   */
  async checkAvailability(): Promise<boolean> {
    try {
      await semanticSearchApi.getStatus()
      this.available = true
      return true
    } catch {
      this.available = false
      return false
    }
  }

  /**
   * Hydrate movie IDs to full Movie objects
   */
  private async hydrateMovies(movieIds: number[]): Promise<Movie[]> {
    try {
      // Fetch all movies in parallel
      const moviePromises = movieIds.map((id) => moviesApi.getMovie(id))
      const responses = await Promise.all(moviePromises)

      // Extract successful responses preserving order
      const movies = responses
        .filter((r) => r.status === 200 && r.data)
        .map((r) => r.data) as Movie[]

      return movies
    } catch (error) {
      logger.error('Failed to hydrate movies:', error)
      return []
    }
  }

  priority = 10 // Higher than built-in (0)
}

export const semanticSearchProvider = new SemanticSearchProvider()
