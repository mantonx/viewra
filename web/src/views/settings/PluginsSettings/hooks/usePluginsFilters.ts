import { useState, useMemo } from 'react'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'
import type { FilterTab, PluginGroupInfo } from '../PluginsSettings.types'

/**
 * Map category IDs to display labels for grouping.
 * Backend returns category IDs (e.g., "search"), we display labels (e.g., "Search").
 */
const categoryLabels: Record<string, string> = {
  search: 'Search',
  recommendations: 'Recommendations',
  enrichers: 'Enrichers',
  providers: 'AI Providers',
  local: 'Local',
  other: 'Other',
}

/**
 * Gets the display label for a category ID.
 */
const getCategoryLabel = (categoryId: string): string => {
  return categoryLabels[categoryId] || 'Other'
}

/**
 * Hook for filtering and grouping plugins.
 * Takes tabs from the backend instead of computing them locally.
 */
export const usePluginsFilters = (plugins: PluginSummary[], tabs: FilterTab[]) => {
  const [activeTabId, setActiveTabId] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Filter plugins based on active tab and search query
  const filteredPlugins = useMemo(() => {
    let result = plugins

    // Apply tab filter based on tab id (tab IDs match category IDs from backend)
    if (activeTabId !== 'all') {
      if (activeTabId === 'disabled') {
        result = result.filter((p) => !p.enabled)
      } else {
        // Tab ID is the same as category ID (e.g., "search", "providers")
        result = result.filter((p) => p.display_category === activeTabId)
      }
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
  }, [plugins, activeTabId, searchQuery])

  // Group filtered plugins by category ID from backend
  const groupedPlugins: PluginGroupInfo[] = useMemo(() => {
    const groups: Record<string, PluginSummary[]> = {}

    for (const plugin of filteredPlugins) {
      // display_category is now a category ID (e.g., "search", "providers")
      const categoryId = plugin.display_category || 'other'
      if (!groups[categoryId]) {
        groups[categoryId] = []
      }
      groups[categoryId].push(plugin)
    }

    // Define category ID order (matches backend Categories priority order)
    const categoryOrder = ['search', 'recommendations', 'enrichers', 'providers', 'local', 'other']

    // Get all unique category IDs, sorted by the predefined order
    const allCategoryIds = Object.keys(groups).sort((a, b) => {
      const aIndex = categoryOrder.indexOf(a)
      const bIndex = categoryOrder.indexOf(b)
      // Categories not in the order go to the end
      const aOrder = aIndex === -1 ? categoryOrder.length : aIndex
      const bOrder = bIndex === -1 ? categoryOrder.length : bIndex
      return aOrder - bOrder
    })

    return allCategoryIds.map((categoryId) => ({
      id: categoryId,
      name: getCategoryLabel(categoryId),
      plugins: groups[categoryId],
    }))
  }, [filteredPlugins])

  return {
    activeTabId,
    setActiveTabId,
    tabs,
    searchQuery,
    setSearchQuery,
    groupedPlugins,
  }
}
