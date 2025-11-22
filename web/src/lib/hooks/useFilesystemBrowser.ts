import { useState, useCallback } from 'react'
import { useGetApiFilesystemBrowse } from '@/lib/api'
import type { GetApiFilesystemBrowseParams, GithubComMantonxViewraInternalDomainLibraryBrowseResult } from '@/lib/api'

/**
 * Hook for browsing the filesystem during library path selection
 * Provides navigation state management and convenience methods
 */
const useFilesystemBrowser = (initialPath?: string) => {
  const [currentPath, setCurrentPath] = useState<string | undefined>(initialPath)

  // Prepare params - pass empty object when currentPath is undefined to use default path
  const params = currentPath !== undefined ? ({ path: currentPath } as GetApiFilesystemBrowseParams) : ({} as GetApiFilesystemBrowseParams)

  // Fetch browse results for current path
  const {
    data: response,
    error,
    isLoading,
    isFetching,
    refetch,
  } = useGetApiFilesystemBrowse(params, {
    query: {
      // Only enable query when we have a path or want to browse from default
      enabled: true,
      // Keep previous data while loading new directory
      placeholderData: (previousData) => previousData,
    },
  })

  // Extract browse result from response
  // Handle both wrapped ({ data, status }) and unwrapped (direct BrowseResult) responses
  const browseResult = response?.status === 200
    ? response.data
    : (response as unknown as GithubComMantonxViewraInternalDomainLibraryBrowseResult | undefined)

  // Navigation methods
  const navigateToPath = useCallback((path: string) => {
    setCurrentPath(path)
  }, [])

  const navigateToParent = useCallback(() => {
    if (browseResult?.parent) {
      setCurrentPath(browseResult.parent)
    }
  }, [browseResult?.parent])

  const navigateToDirectory = useCallback(
    (directoryPath: string) => {
      setCurrentPath(directoryPath)
    },
    []
  )

  const reset = useCallback(() => {
    setCurrentPath(initialPath)
  }, [initialPath])

  return {
    // Data
    browseResult,
    currentPath,
    error,

    // Loading states
    isLoading,
    isFetching,

    // Navigation methods
    navigateToPath,
    navigateToParent,
    navigateToDirectory,
    reset,
    refetch,

    // Computed states
    canNavigateUp: !!browseResult?.parent,
    isAtRoot: browseResult?.is_root ?? false,
    directories: browseResult?.directories ?? [],
  }
}

export { useFilesystemBrowser }
