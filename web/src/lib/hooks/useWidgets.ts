import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { customInstance } from '@/lib/api/mutator'
import type {
  HomeResponse,
  SuggestionsResponse,
  SearchProviderInfo,
} from '@/components/home/widgets/widget.types'

/**
 * Widget preference for reordering/hiding
 */
export interface WidgetPreference {
  id: string
  position: number
  hidden: boolean
}

/**
 * Preferences update request
 */
export interface PreferencesUpdateRequest {
  sections: WidgetPreference[]
}

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
 * Note: Plugin custom routes use /api/plugin/:plugin_id/* (singular)
 */
const fetchSuggestions = async (limit: number = 6): Promise<SuggestionsResponse> => {
  const response = await customInstance<{ data: SuggestionsResponse }>({
    url: buildUrl('/api/plugin/semantic-search/suggestions', { limit }),
    method: 'GET',
  })
  return response.data
}

/**
 * Fetch search provider info
 * Note: Plugin custom routes use /api/plugin/:plugin_id/* (singular)
 */
const fetchSearchProviderInfo = async (): Promise<SearchProviderInfo> => {
  const response = await customInstance<{ data: SearchProviderInfo }>({
    url: '/api/plugin/semantic-search/search/info',
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
    suggestions: suggestions?.Suggestions ?? [],
    search_url: '/api/search',
  }

  return {
    data: searchHeroData,
    isLoading,
    error,
  }
}

/**
 * Fetch widget preferences
 */
const fetchWidgetPreferences = async (): Promise<WidgetPreference[]> => {
  const response = await customInstance<{ data: WidgetPreference[] }>({
    url: '/api/home/preferences',
    method: 'GET',
  })
  return response.data
}

/**
 * Update widget preferences
 */
const updateWidgetPreferences = async (request: PreferencesUpdateRequest): Promise<void> => {
  await customInstance({
    url: '/api/home/preferences',
    method: 'PUT',
    data: request,
  })
}

/**
 * Reset widget preferences to defaults
 */
const resetWidgetPreferences = async (): Promise<void> => {
  await customInstance({
    url: '/api/home/preferences',
    method: 'DELETE',
  })
}

/**
 * Query options for widget preferences
 */
export const getWidgetPreferencesQueryOptions = () => ({
  queryKey: ['home', 'preferences'],
  queryFn: fetchWidgetPreferences,
  staleTime: 5 * 60 * 1000, // 5 minutes
  gcTime: 30 * 60 * 1000, // 30 minutes
})

/**
 * Hook to fetch widget preferences
 */
export const useWidgetPreferences = () => {
  return useQuery(getWidgetPreferencesQueryOptions())
}

/**
 * Hook to update widget preferences
 */
export const useUpdateWidgetPreferences = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateWidgetPreferences,
    onSuccess: () => {
      // Invalidate both preferences and home sections
      queryClient.invalidateQueries({ queryKey: ['home'] })
    },
  })
}

/**
 * Hook to reset widget preferences
 */
export const useResetWidgetPreferences = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: resetWidgetPreferences,
    onSuccess: () => {
      // Invalidate both preferences and home sections
      queryClient.invalidateQueries({ queryKey: ['home'] })
    },
  })
}
