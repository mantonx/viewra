import { useQuery } from '@tanstack/react-query'
import { customInstance } from '@/lib/api/mutator'
import type {
  HomeResponse,
  SuggestionsResponse,
  SearchProviderInfo,
} from '@/components/home/widgets/widget.types'

/**
 * Helper to build URL with query params
 */
const buildUrl = (base: string, params?: Record<string, string | number>): string => {
  if (!params || Object.keys(params).length === 0) {
    return base
  }
  const searchParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null) {
      searchParams.set(key, String(value))
    }
  }
  return `${base}?${searchParams.toString()}`
}

/**
 * Fetch home screen sections from the API
 */
const fetchHomeSections = async (clientType: string = 'web'): Promise<HomeResponse> => {
  const response = await customInstance<{ data: HomeResponse }>({
    url: buildUrl('/api/home', { client_type: clientType }),
    method: 'GET',
  })
  return response.data
}

/**
 * Fetch search suggestions from the semantic-search plugin
 */
const fetchSuggestions = async (limit: number = 6): Promise<SuggestionsResponse> => {
  const response = await customInstance<{ data: SuggestionsResponse }>({
    url: buildUrl('/api/plugins/semantic-search/suggestions', { limit }),
    method: 'GET',
  })
  return response.data
}

/**
 * Fetch search provider info
 */
const fetchSearchProviderInfo = async (): Promise<SearchProviderInfo> => {
  const response = await customInstance<{ data: SearchProviderInfo }>({
    url: '/api/plugins/semantic-search/search/info',
    method: 'GET',
  })
  return response.data
}

/**
 * Query options for home sections
 */
export const getHomeQueryOptions = (clientType: string = 'web') => ({
  queryKey: ['home', 'sections', clientType],
  queryFn: () => fetchHomeSections(clientType),
  staleTime: 60 * 1000, // 1 minute
  gcTime: 5 * 60 * 1000, // 5 minutes
})

/**
 * Query options for search suggestions
 */
export const getSuggestionsQueryOptions = (limit: number = 6) => ({
  queryKey: ['home', 'suggestions', limit],
  queryFn: () => fetchSuggestions(limit),
  staleTime: 60 * 1000, // 1 minute - suggestions change with context
  gcTime: 5 * 60 * 1000, // 5 minutes
})

/**
 * Query options for search provider info
 */
export const getSearchProviderInfoQueryOptions = () => ({
  queryKey: ['home', 'search-provider-info'],
  queryFn: fetchSearchProviderInfo,
  staleTime: 5 * 60 * 1000, // 5 minutes - rarely changes
  gcTime: 30 * 60 * 1000, // 30 minutes
})

/**
 * Hook to fetch home screen sections
 *
 * Fetches widget sections from the home API and returns them
 * grouped by location for easy rendering.
 */
export const useHomeSections = (clientType: string = 'web') => {
  return useQuery(getHomeQueryOptions(clientType))
}

/**
 * Hook to fetch search suggestions
 *
 * Fetches contextual search suggestions from the semantic-search plugin.
 * Suggestions are personalized based on time, weather, and user preferences.
 */
export const useSuggestions = (limit: number = 6) => {
  return useQuery(getSuggestionsQueryOptions(limit))
}

/**
 * Hook to fetch search provider info
 *
 * Returns information about the active search provider (semantic search).
 */
export const useSearchProviderInfo = () => {
  return useQuery(getSearchProviderInfoQueryOptions())
}

/**
 * Hook to fetch suggestions and build SearchHeroData
 *
 * Combines suggestions with default search configuration
 * for use with the SearchHero component.
 */
export const useSearchHeroData = () => {
  const { data: suggestions, isLoading, error } = useSuggestions()

  const searchHeroData = {
    placeholder: 'Search your library...',
    suggestions: suggestions?.suggestions ?? [],
    search_url: '/api/search',
  }

  return {
    data: searchHeroData,
    isLoading,
    error,
  }
}
