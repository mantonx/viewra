import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { extractLibraries, getLibraryId, filterLibrariesByType } from '@/lib/utils/api'

export type LibraryType = 'movies' | 'tv' | 'music'

/**
 * Reusable hook for library filtering across media browsing pages.
 * Manages library selection state and provides filtered library list and active library ID.
 *
 * @param type - The type of media library to filter for ('movies', 'tv', or 'music')
 * @returns Library filtering state and utilities
 *
 * @example
 * ```tsx
 * const { selectedLibrary, setSelectedLibrary, libraries, libraryId } = useLibraryFilter('movies')
 * ```
 */
export const useLibraryFilter = (type: LibraryType) => {
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')

  // Fetch all libraries
  const { data: librariesData, isSuccess } = useQuery(getGetApiLibrariesQueryOptions())
  const allLibraries = extractLibraries(librariesData)

  // Filter to libraries of the specified type
  const libraries = filterLibrariesByType(allLibraries, type)

  // Get the active library ID (defaults to first library of type when 'all' is selected)
  const libraryId = getLibraryId(selectedLibrary, allLibraries, type)

  return {
    /** Currently selected library ID or 'all' */
    selectedLibrary,
    /** Update the selected library */
    setSelectedLibrary,
    /** Filtered list of libraries for the specified type */
    libraries,
    /** Active library ID to use for queries */
    libraryId,
    /** Whether libraries have loaded successfully */
    isReady: isSuccess,
  }
}
