import { useState, useMemo } from 'react'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'
import type { FilterTab, TabCounts, PluginGroupInfo } from '../PluginsSettings.types'

/**
 * Normalizes categories from the API response.
 * Handles malformed JSON strings like '["enricher"]' being returned as array elements.
 */
const normalizeCategories = (categories: string[] | undefined): string[] => {
  if (!categories || categories.length === 0) {
    return []
  }

  const result: string[] = []
  for (const cat of categories) {
    // Handle JSON array strings like '["enricher"]'
    if (cat.startsWith('[')) {
      try {
        const parsed = JSON.parse(cat)
        if (Array.isArray(parsed)) {
          result.push(...parsed)
          continue
        }
      } catch {
        // Not valid JSON, continue with normal handling
      }
    }
    // Clean up any quotes or brackets
    const cleaned = cat.replace(/[[\]"]/g, '').trim()
    if (cleaned) {
      result.push(cleaned)
    }
  }
  return result
}

/**
 * Determines the category for a plugin based on its capabilities and categories.
 * Note: Disabled plugins stay in their original category (they don't move to "Other").
 */
const getPluginCategory = (plugin: PluginSummary): string => {
  const capabilities = plugin.capabilities ?? []
  const categories = normalizeCategories(plugin.categories)
  const isBuiltin = plugin.is_builtin ?? false

  // Builtin plugins go to Local category
  if (isBuiltin) {
    return 'Local'
  }

  // Check if it's a provider (has provider capability)
  if (capabilities.some((c) => c.startsWith('provider'))) {
    return 'AI Providers'
  }

  // Check categories for enrichers (TMDb, MusicBrainz, Semantic Search)
  if (categories.includes('enricher')) {
    return 'Enrichers'
  }

  // Check if it's a search plugin
  if (capabilities.includes('search') || capabilities.includes('semantic_search')) {
    return 'Search'
  }

  // Default category
  return 'Other'
}

/**
 * Determines if a plugin is considered an "enricher" type for filtering.
 */
const isEnricher = (plugin: PluginSummary): boolean => {
  const categories = normalizeCategories(plugin.categories)
  return categories.includes('enricher')
}

/**
 * Determines if a plugin is considered a "provider" type for filtering.
 */
const isProvider = (plugin: PluginSummary): boolean => {
  const capabilities = plugin.capabilities ?? []
  return capabilities.some((c) => c.startsWith('provider'))
}

/**
 * Hook for filtering and grouping plugins.
 */
export const usePluginsFilters = (plugins: PluginSummary[]) => {
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Calculate tab counts
  const tabCounts: TabCounts = useMemo(() => {
    return {
      all: plugins.length,
      enrichers: plugins.filter(isEnricher).length,
      providers: plugins.filter(isProvider).length,
      disabled: plugins.filter((p) => !p.enabled).length,
    }
  }, [plugins])

  // Filter plugins based on active tab and search query
  const filteredPlugins = useMemo(() => {
    let result = plugins

    // Apply tab filter
    switch (activeTab) {
      case 'enrichers':
        result = result.filter(isEnricher)
        break
      case 'providers':
        result = result.filter(isProvider)
        break
      case 'disabled':
        result = result.filter((p) => !p.enabled)
        break
    }

    // Apply search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      result = result.filter((plugin) => {
        const name = String(plugin.meta?.displayName ?? plugin.name ?? '').toLowerCase()
        const description = (plugin.description ?? '').toLowerCase()
        const capabilities = (plugin.capabilities ?? []).join(' ').toLowerCase()
        return name.includes(query) || description.includes(query) || capabilities.includes(query)
      })
    }

    return result
  }, [plugins, activeTab, searchQuery])

  // Group filtered plugins by category
  const groupedPlugins: PluginGroupInfo[] = useMemo(() => {
    const groups: Record<string, PluginSummary[]> = {}

    for (const plugin of filteredPlugins) {
      const category = getPluginCategory(plugin)
      if (!groups[category]) {
        groups[category] = []
      }
      groups[category].push(plugin)
    }

    // Define category order
    const categoryOrder = ['Enrichers', 'AI Providers', 'Local', 'Other']

    return categoryOrder
      .filter((category) => groups[category]?.length > 0)
      .map((category) => ({
        id: category.toLowerCase().replace(/\s+/g, '-'),
        name: category,
        plugins: groups[category],
      }))
  }, [filteredPlugins])

  return {
    activeTab,
    setActiveTab,
    tabCounts,
    searchQuery,
    setSearchQuery,
    groupedPlugins,
  }
}
