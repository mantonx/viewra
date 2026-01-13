/**
 * Search Abstraction Layer
 *
 * Core search types with ZERO knowledge of specific plugins.
 * This layer enables plugins to extend search without coupling core code.
 *
 * Principle: Core never names plugins. It only knows:
 * - "There's a search capability"
 * - "Here's a slot for enhancement UI"
 */

import type { ReactNode } from 'react'

/**
 * Generic search options accepted by any search provider
 */
export interface SearchOptions {
  /** Media types to filter by (empty = all types) */
  entityTypes?: string[]

  /** Maximum results to return */
  limit?: number

  /** Additional provider-specific options (opaque to core) */
  extra?: Record<string, unknown>
}

/**
 * Generic search result returned by any search provider
 */
export interface SearchResult<T = unknown> {
  /** The actual search results */
  items: T[]

  /** Total number of matches */
  total: number

  /** Optional UI enhancement from plugin (displayed above results) */
  enhancement?: ReactNode

  /** Whether fallback search was used (no plugin available) */
  fallback?: boolean
}

/**
 * SearchProvider interface - implemented by plugins
 *
 * Core code depends on this interface, NOT on specific implementations.
 * This follows dependency inversion principle.
 */
export interface SearchProvider<T = unknown> {
  /**
   * Perform search with optional enhancements
   */
  search(query: string, options?: SearchOptions): Promise<SearchResult<T>>

  /**
   * Check if this provider is currently available
   */
  isAvailable(): boolean

  /**
   * Provider priority (higher = preferred, optional)
   */
  priority?: number
}
