import { useCallback, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { semanticSearchApi, type SearchParams, type SemanticSearchResponse } from '../api/client'
import { semanticSearchProvider } from '../provider'

/**
 * Hook to check if semantic search plugin is available
 */
export function useSemanticSearchAvailable() {
  return useQuery({
    queryKey: ['semantic-search', 'available'],
    queryFn: async () => {
      try {
        const status = await semanticSearchApi.getStatus()
        return { available: true, status }
      } catch (error) {
        return { available: false, status: null }
      }
    },
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
    retry: false, // Don't retry if plugin not available
  })
}

/**
 * Hook to perform semantic search
 */
export function useSemanticSearch(
  params: SearchParams,
  options?: {
    enabled?: boolean
  }
) {
  return useQuery<SemanticSearchResponse>({
    queryKey: ['semantic-search', 'search', params],
    queryFn: () => semanticSearchApi.search(params),
    enabled: options?.enabled !== false && !!params.query && params.query.length >= 2,
    staleTime: 30 * 1000, // Cache for 30 seconds
  })
}

/**
 * Hook to manage semantic search with chip removal support.
 *
 * This hook manages the excluded intents state and automatically
 * wires up the chip removal callback on the search provider.
 *
 * @example
 * ```tsx
 * const {
 *   excludedIntents,
 *   removeChip,
 *   clearExcludedIntents,
 *   searchOptions,
 * } = useSemanticSearchChips()
 *
 * // Pass searchOptions.extra to useSearch
 * const { data } = useSearch(query, {
 *   entityTypes: ['movie'],
 *   extra: searchOptions.extra,
 * })
 * ```
 */
export function useSemanticSearchChips() {
  const [excludedIntents, setExcludedIntents] = useState<string[]>([])

  // Callback when a chip is removed from the UI
  const handleChipRemove = useCallback((_chipId: string, chipType: string) => {
    setExcludedIntents((prev) => {
      if (prev.includes(chipType)) {
        return prev
      }
      return [...prev, chipType]
    })
  }, [])

  // Register callback on provider mount, unregister on unmount
  useEffect(() => {
    semanticSearchProvider.setOnChipRemoveCallback(handleChipRemove)
    return () => {
      semanticSearchProvider.setOnChipRemoveCallback(null)
    }
  }, [handleChipRemove])

  // Function to manually remove a chip/intent type
  const removeChip = useCallback((chipType: string) => {
    setExcludedIntents((prev) => {
      if (prev.includes(chipType)) {
        return prev
      }
      return [...prev, chipType]
    })
  }, [])

  // Function to clear all excluded intents
  const clearExcludedIntents = useCallback(() => {
    setExcludedIntents([])
  }, [])

  // Function to restore a specific intent
  const restoreChip = useCallback((chipType: string) => {
    setExcludedIntents((prev) => prev.filter((t) => t !== chipType))
  }, [])

  // Build search options to pass to useSearch
  const searchOptions = {
    extra: {
      excludeIntents: excludedIntents.length > 0 ? excludedIntents : undefined,
    },
  }

  return {
    /** Currently excluded intent types */
    excludedIntents,
    /** Remove a chip by its type (e.g., "decade", "person") */
    removeChip,
    /** Restore a previously removed chip type */
    restoreChip,
    /** Clear all excluded intents */
    clearExcludedIntents,
    /** Search options to spread into useSearch options */
    searchOptions,
  }
}

/**
 * Hook to find similar items
 */
export function useSimilarItems(
  entityType: string,
  entityId: number,
  limit?: number,
  options?: {
    enabled?: boolean
  }
) {
  return useQuery<SemanticSearchResponse>({
    queryKey: ['semantic-search', 'similar', entityType, entityId, limit],
    queryFn: () => semanticSearchApi.findSimilar(entityType, entityId, limit),
    enabled: options?.enabled !== false && !!entityId,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  })
}
