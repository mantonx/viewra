/**
 * Semantic Search Plugin
 *
 * This plugin enhances search with:
 * - Intent detection and chips
 * - Semantic understanding
 * - Result explanations
 *
 * The plugin registers itself with the core search system.
 * Core code never imports this module directly.
 */

import { registerSearchProvider } from '@/lib/search'
import { semanticSearchProvider } from './provider'

// Register this plugin as a search provider
registerSearchProvider(semanticSearchProvider)

// Check availability asynchronously
semanticSearchProvider.checkAvailability()

// Export for testing/debugging only
export { semanticSearchProvider }

// Export hooks for use in components
export { useSemanticSearchChips } from './hooks/useSemanticSearch'
