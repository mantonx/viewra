import { useState, useEffect, useCallback, useRef } from 'react'
import { pluginApi } from '@/lib/api/pluginApi'

/**
 * Autocomplete suggestion from the semantic-search plugin
 */
export interface AutocompleteSuggestion {
  type: 'title' | 'person' | 'recent'
  text: string
  entity_id: number
  subtype?: string // 'movie', 'tv_show', 'director', 'actor', etc.
  year?: number
  popularity?: number
}

interface AutocompleteResponse {
  suggestions: AutocompleteSuggestion[]
  query: string
  took_ms: number
}

interface UseAutocompleteOptions {
  /** Debounce delay in ms (default: 250) */
  debounceMs?: number
  /** Maximum suggestions to fetch (default: 8) */
  limit?: number
  /** Filter by type: 'all', 'titles', 'people' (default: 'all') */
  types?: 'all' | 'titles' | 'people'
  /** Minimum query length to trigger search (default: 2) */
  minLength?: number
}

interface UseAutocompleteReturn {
  suggestions: AutocompleteSuggestion[]
  isLoading: boolean
  error: string | null
  /** Clear suggestions and reset state */
  clear: () => void
}

/**
 * Hook for fetching autocomplete suggestions from the semantic-search plugin.
 * Includes debouncing and automatic cleanup.
 *
 * @example
 * const { suggestions, isLoading } = useAutocomplete(query, { debounceMs: 250 })
 */
export const useAutocomplete = (
  query: string,
  options: UseAutocompleteOptions = {}
): UseAutocompleteReturn => {
  const { debounceMs = 250, limit = 8, types = 'all', minLength = 2 } = options

  const [suggestions, setSuggestions] = useState<AutocompleteSuggestion[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Track the latest query to avoid race conditions
  const latestQueryRef = useRef(query)
  latestQueryRef.current = query

  // Abort controller for cancelling in-flight requests
  const abortControllerRef = useRef<AbortController | null>(null)

  const clear = useCallback(() => {
    setSuggestions([])
    setError(null)
    setIsLoading(false)
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
    }
  }, [])

  useEffect(() => {
    // Clear if query is too short
    if (query.length < minLength) {
      setSuggestions([])
      setError(null)
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setError(null)

    // Debounce the API call
    const timeoutId = setTimeout(async () => {
      // Cancel any previous request
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
      }
      abortControllerRef.current = new AbortController()

      try {
        const response = await pluginApi.get<AutocompleteResponse>(
          'semantic-search',
          '/autocomplete',
          {
            q: query,
            limit: String(limit),
            types,
          }
        )

        // Only update if this is still the latest query
        if (latestQueryRef.current === query) {
          setSuggestions(response.suggestions || [])
          setError(null)
        }
      } catch (err) {
        // Ignore abort errors
        if (err instanceof Error && err.name === 'AbortError') {
          return
        }

        // Only update error if this is still the latest query
        if (latestQueryRef.current === query) {
          setError(err instanceof Error ? err.message : 'Failed to fetch suggestions')
          setSuggestions([])
        }
      } finally {
        // Only update loading if this is still the latest query
        if (latestQueryRef.current === query) {
          setIsLoading(false)
        }
      }
    }, debounceMs)

    return () => {
      clearTimeout(timeoutId)
    }
  }, [query, debounceMs, limit, types, minLength])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
      }
    }
  }, [])

  return { suggestions, isLoading, error, clear }
}
