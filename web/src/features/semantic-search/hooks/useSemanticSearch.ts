import { useQuery } from '@tanstack/react-query'
import { semanticSearchApi, type SearchParams, type SemanticSearchResponse } from '../api/client'

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
