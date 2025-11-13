/**
 * Custom hook for FilesystemBrowser preferences using generic storage utilities
 */
import { useState, useCallback } from 'react'
import { getFromStorage, saveToStorage, addToRecentList, getRecentList, clearRecentList } from '@/lib/utils/browserStorage'

const RECENT_PATHS_KEY = 'viewra-fs-browser-recent-paths'
const SORT_PREFERENCE_KEY = 'viewra-fs-browser-sort'
const MAX_RECENT_PATHS = 5

export type SortBy = 'name' | 'modified'
export type SortOrder = 'asc' | 'desc'

export interface SortPreference {
  sortBy: SortBy
  sortOrder: SortOrder
}

/**
 * Hook for managing FilesystemBrowser preferences (recent paths and sorting)
 */
export const useBrowserPreferences = () => {
  // Recent paths state
  const [recentPaths, setRecentPaths] = useState<string[]>(() =>
    getRecentList<string>(RECENT_PATHS_KEY, [])
  )

  // Sort preference state
  const [sortPreference, setSortPreference] = useState<SortPreference>(() =>
    getFromStorage<SortPreference>(SORT_PREFERENCE_KEY, { sortBy: 'name', sortOrder: 'asc' })
  )

  // Add a path to recent paths
  const addRecentPath = useCallback((path: string) => {
    addToRecentList(RECENT_PATHS_KEY, path, MAX_RECENT_PATHS)
    setRecentPaths(getRecentList<string>(RECENT_PATHS_KEY, []))
  }, [])

  // Clear recent paths
  const clearRecent = useCallback(() => {
    clearRecentList(RECENT_PATHS_KEY)
    setRecentPaths([])
  }, [])

  // Update sort preference
  const updateSortPreference = useCallback((sortBy: SortBy, sortOrder: SortOrder) => {
    const newPreference = { sortBy, sortOrder }
    saveToStorage(SORT_PREFERENCE_KEY, newPreference)
    setSortPreference(newPreference)
  }, [])

  return {
    recentPaths,
    addRecentPath,
    clearRecent,
    sortPreference,
    updateSortPreference,
  }
}
