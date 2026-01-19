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

/**
 * Callback type for chip removal events.
 * Used to notify parent components that a chip was removed,
 * allowing them to trigger a new search with updated exclusions.
 */
export type OnChipRemoveCallback = (chipId: string, chipType: string) => void

class SemanticSearchProvider implements SearchProvider<Movie> {
  private available: boolean | null = null
  private onChipRemoveCallback: OnChipRemoveCallback | null = null

  /**
   * Register a callback to be notified when a chip is removed.
   * This allows the search hook to trigger a re-search with exclusions.
   */
  setOnChipRemoveCallback(callback: OnChipRemoveCallback | null) {
    this.onChipRemoveCallback = callback
  }

  async search(query: string, options?: SearchOptions): Promise<SearchResult<Movie>> {
    try {
      // Extract excluded intents from options
      const excludeIntents = (options?.extra?.excludeIntents as string[]) || []

      // Call semantic search API
      const response = await semanticSearchApi.search({
        query,
        entity_types: options?.entityTypes || ['movie'],
        limit: options?.limit || 100,
        exclude_intents: excludeIntents.length > 0 ? excludeIntents : undefined,
      })

      // Hydrate entity IDs to full Movie objects
      const movies = await this.hydrateMovies(response.results.map((r) => r.entity_id))

      // Filter out chips that have been excluded
      const visibleChips = response.intent_chips?.filter(
        (chip) => !excludeIntents.includes(chip.type)
      ) || []

      // Build enhancement UI with intent chips
      const enhancement = visibleChips.length > 0 ? (
        <IntentChipsBar
          chips={visibleChips}
          onRemoveChip={(chipId) => {
            const chip = visibleChips.find((c) => c.id === chipId)
            if (chip && this.onChipRemoveCallback) {
              logger.debug('Chip removed:', chip.type, chip.value)
              this.onChipRemoveCallback(chipId, chip.type)
            }
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
