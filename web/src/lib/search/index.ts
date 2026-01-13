/**
 * Search Abstraction Layer - Public API
 *
 * This is the ONLY search module that core code should import.
 * It provides plugin-agnostic search functionality.
 */

export type { SearchProvider, SearchOptions, SearchResult } from './types'
export { registerSearchProvider, getSearchProvider } from './registry'
export { useSearch, useHasEnhancedSearch } from './hooks'
export { builtinSearchProvider } from './builtin'
