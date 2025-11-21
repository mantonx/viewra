/**
 * useMediaLibraryState Hook
 *
 * Unified state management for media library pages (movies, TV, music).
 * Handles URL synchronization for search, sort, view mode, and advanced filters.
 *
 * Features:
 * - URL-based state persistence (shareable URLs, back/forward navigation)
 * - Intelligent defaults (undefined = default value, cleaner URLs)
 * - Type-safe filter state management
 * - Centralized update logic (DRY principle)
 */

import { useMemo, useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import type { ViewMode, FilterState } from '@/components/common'

export interface MediaLibrarySearchParams {
  id?: number
  q?: string
  sort?: string
  genres?: string
  yearMin?: number
  yearMax?: number
  qualities?: string
  watched?: string
  view?: ViewMode
}

export interface MediaLibraryState {
  // Current state
  searchQuery: string
  sort: string
  viewMode: ViewMode
  filters: FilterState

  // API-formatted values
  apiSort: string

  // Update handlers
  updateSearch: (q: string) => void
  updateSort: (sort: string) => void
  updateViewMode: (view: ViewMode) => void
  updateFilters: (filters: FilterState) => void
  updateAll: (updates: Partial<MediaLibrarySearchParams>) => void
}

interface UseMediaLibraryStateOptions {
  routePath: '/movies' | '/tv' | '/music'
  searchParams: MediaLibrarySearchParams
  defaultSort?: string
  defaultView?: ViewMode
}

/**
 * Unified hook for managing media library state across movies, TV, and music pages.
 * Eliminates duplication of URL state management logic.
 */
export const useMediaLibraryState = ({
  routePath,
  searchParams,
  defaultSort = 'title-asc',
  defaultView = 'grid',
}: UseMediaLibraryStateOptions): MediaLibraryState => {
  const navigate = useNavigate()

  // Parse current state from URL
  const searchQuery = searchParams.q || ''
  const sort = searchParams.sort || defaultSort
  const viewMode = searchParams.view || defaultView

  // Parse advanced filters from URL (for movies, optional for TV/music)
  const filters: FilterState = useMemo(() => ({
    genres: searchParams.genres ? searchParams.genres.split(',') : [],
    yearMin: searchParams.yearMin,
    yearMax: searchParams.yearMax,
    qualities: searchParams.qualities ? searchParams.qualities.split(',') : [],
    watchedFilter: (searchParams.watched as 'all' | 'watched' | 'unwatched') || 'all',
  }), [searchParams.genres, searchParams.yearMin, searchParams.yearMax, searchParams.qualities, searchParams.watched])

  // Convert sort format from URL (title-asc) to API format (title_asc)
  const apiSort = sort.replace(/-/g, '_')

  // Centralized URL update function
  const updateAll = useCallback((updates: Partial<MediaLibrarySearchParams>) => {
    navigate({
      to: routePath,
      search: {
        id: undefined, // Clear ID when updating filters
        q: searchParams.q,
        sort: searchParams.sort,
        genres: searchParams.genres,
        yearMin: searchParams.yearMin,
        yearMax: searchParams.yearMax,
        qualities: searchParams.qualities,
        watched: searchParams.watched,
        view: searchParams.view,
        ...updates,
      },
      replace: true,
    })
  }, [navigate, routePath, searchParams])

  // Individual update handlers (convenience wrappers)
  const updateSearch = useCallback((q: string) => {
    updateAll({ q: q || undefined })
  }, [updateAll])

  const updateSort = useCallback((newSort: string) => {
    updateAll({ sort: newSort === defaultSort ? undefined : newSort })
  }, [updateAll, defaultSort])

  const updateViewMode = useCallback((view: ViewMode) => {
    updateAll({ view: view === defaultView ? undefined : view })
  }, [updateAll, defaultView])

  const updateFilters = useCallback((newFilters: FilterState) => {
    updateAll({
      genres: newFilters.genres && newFilters.genres.length > 0
        ? newFilters.genres.join(',')
        : undefined,
      yearMin: newFilters.yearMin,
      yearMax: newFilters.yearMax,
      qualities: newFilters.qualities && newFilters.qualities.length > 0
        ? newFilters.qualities.join(',')
        : undefined,
      watched: newFilters.watchedFilter !== 'all'
        ? newFilters.watchedFilter
        : undefined,
    })
  }, [updateAll])

  return {
    searchQuery,
    sort,
    viewMode,
    filters,
    apiSort,
    updateSearch,
    updateSort,
    updateViewMode,
    updateFilters,
    updateAll,
  }
}
