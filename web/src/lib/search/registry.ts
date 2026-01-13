/**
 * Plugin Registry
 *
 * Dynamically detects and manages search providers.
 * NO hardcoded plugin names - plugins register themselves.
 */

import type { SearchProvider } from './types'

interface ProviderRegistry {
  providers: SearchProvider[]
}

// Global registry (populated by plugins at init time)
const registry: ProviderRegistry = {
  providers: [],
}

/**
 * Register a search provider (called by plugins during initialization)
 *
 * @example
 * ```typescript
 * // In plugin's index.ts
 * registerSearchProvider(mySearchProvider)
 * ```
 */
export function registerSearchProvider(provider: SearchProvider): void {
  registry.providers.push(provider)

  // Sort by priority (higher first)
  registry.providers.sort((a, b) => (b.priority || 0) - (a.priority || 0))
}

/**
 * Get the best available search provider
 *
 * Returns the highest-priority available provider.
 * If no providers available, returns null.
 */
export function getSearchProvider(): SearchProvider | null {
  // Find first available provider
  const provider = registry.providers.find((p) => p.isAvailable())

  return provider || null
}

/**
 * Get all registered providers (for debugging)
 */
export function getAllProviders(): SearchProvider[] {
  return [...registry.providers]
}

/**
 * Clear all providers (for testing)
 */
export function clearProviders(): void {
  registry.providers = []
}
