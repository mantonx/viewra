import { useState, useEffect, useCallback, useMemo } from 'react'
import { useFilesystemBrowser } from '@/lib/hooks/useFilesystemBrowser'
import { useBrowserPreferences } from '@/lib/hooks/useBrowserPreferences'
import { Modal, ModalContent, ModalFooter } from '@/components/ui/Modal/Modal'
import { Button } from '@/components/ui/Button/Button'
import { QuickAccessBar } from './QuickAccessBar'
import { BreadcrumbNav } from './BreadcrumbNav'
import { PathInput } from './PathInput'
import { DirectorySearch } from './DirectorySearch'
import { DirectoryTable } from './DirectoryTable'
import { ErrorDisplay } from './ErrorDisplay'
import type { FilesystemBrowserProps } from './FilesystemBrowser.types'
import type { SortBy } from '@/lib/hooks/useBrowserPreferences'

const FilesystemBrowser = ({ isOpen, onClose, onSelect, initialPath }: FilesystemBrowserProps) => {
  // Hooks
  const {
    currentPath,
    error,
    isLoading,
    navigateToParent,
    navigateToDirectory,
    canNavigateUp,
    directories,
    refetch,
  } = useFilesystemBrowser(initialPath)

  const {
    recentPaths,
    addRecentPath,
    clearRecent,
    sortPreference,
    updateSortPreference,
  } = useBrowserPreferences()

  // Local state
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [announcement, setAnnouncement] = useState('')

  // Filtered and sorted directories
  const filteredAndSortedDirectories = useMemo(() => {
    const filtered = searchQuery
      ? directories.filter((dir) =>
          dir.name?.toLowerCase().includes(searchQuery.toLowerCase())
        )
      : directories

    return [...filtered].sort((a, b) => {
      if (sortPreference.sortBy === 'name') {
        const nameA = a.name?.toLowerCase() || ''
        const nameB = b.name?.toLowerCase() || ''
        return sortPreference.sortOrder === 'asc'
          ? nameA.localeCompare(nameB)
          : nameB.localeCompare(nameA)
      } else {
        const dateA = a.modified_at ? new Date(a.modified_at).getTime() : 0
        const dateB = b.modified_at ? new Date(b.modified_at).getTime() : 0
        return sortPreference.sortOrder === 'asc' ? dateA - dateB : dateB - dateA
      }
    })
  }, [directories, searchQuery, sortPreference])

  // Event handlers
  const handleNavigateToDirectory = useCallback((path: string) => {
    navigateToDirectory(path)
    addRecentPath(path)
  }, [navigateToDirectory, addRecentPath])

  const handleSelect = () => {
    if (currentPath) {
      addRecentPath(currentPath)
      onSelect(currentPath)
      onClose()
    }
  }

  const handleSort = (column: SortBy) => {
    const newSortOrder =
      sortPreference.sortBy === column
        ? sortPreference.sortOrder === 'asc'
          ? 'desc'
          : 'asc'
        : 'asc'
    updateSortPreference(column, newSortOrder)
  }

  // Reset selected index when directories or search changes
  useEffect(() => {
    setSelectedIndex(0)
  }, [directories, searchQuery])

  // Announce directory changes to screen readers
  useEffect(() => {
    if (currentPath && isOpen) {
      setAnnouncement(
        `Current directory: ${currentPath}. ${filteredAndSortedDirectories.length} ${
          filteredAndSortedDirectories.length === 1 ? 'directory' : 'directories'
        } available.`
      )
    }
  }, [currentPath, isOpen, filteredAndSortedDirectories.length])

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen) {
      return
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't interfere with input fields
      if (document.activeElement?.tagName === 'INPUT') {
        return
      }

      switch (e.key) {
        case 'Escape':
          e.preventDefault()
          onClose()
          break

        case 'ArrowDown':
          e.preventDefault()
          setSelectedIndex((prev) => Math.min(prev + 1, filteredAndSortedDirectories.length - 1))
          break

        case 'ArrowUp':
          e.preventDefault()
          setSelectedIndex((prev) => Math.max(prev - 1, 0))
          break

        case 'Enter':
          e.preventDefault()
          if (filteredAndSortedDirectories.length > 0 && filteredAndSortedDirectories[selectedIndex]?.readable) {
            const selectedDir = filteredAndSortedDirectories[selectedIndex]
            if (selectedDir.path) {
              handleNavigateToDirectory(selectedDir.path)
            }
          }
          break

        case 'Backspace':
          e.preventDefault()
          if (canNavigateUp) {
            navigateToParent()
          }
          break
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose, filteredAndSortedDirectories, selectedIndex, handleNavigateToDirectory, navigateToParent, canNavigateUp])

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Browse Folders" size="lg" aria-label="Filesystem browser">
      {/* Screen reader announcements */}
      <div className="sr-only" role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </div>

      <ModalContent className="min-h-[400px] flex flex-col" role="main">
        {/* Quick Access */}
        <QuickAccessBar
          onNavigate={handleNavigateToDirectory}
          recentPaths={recentPaths}
          onClearRecent={clearRecent}
          isLoading={isLoading}
        />

        {/* Breadcrumb Navigation */}
        <BreadcrumbNav
          currentPath={currentPath ?? null}
          onNavigate={handleNavigateToDirectory}
          onNavigateUp={navigateToParent}
          canNavigateUp={canNavigateUp}
          isLoading={isLoading}
        />

        {/* Path Input */}
        <PathInput onNavigate={handleNavigateToDirectory} isLoading={isLoading} />

        {/* Error Display */}
        {error && (
          <ErrorDisplay
            error={error}
            onRetry={refetch}
            onNavigateUp={navigateToParent}
            canNavigateUp={canNavigateUp}
          />
        )}

        {/* Search */}
        {!isLoading && !error && directories.length > 0 && (
          <DirectorySearch
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            resultsCount={filteredAndSortedDirectories.length}
            totalCount={directories.length}
          />
        )}

        {/* Directory Table */}
        {!error && (
          <DirectoryTable
            directories={filteredAndSortedDirectories}
            isLoading={isLoading}
            sortBy={sortPreference.sortBy}
            sortOrder={sortPreference.sortOrder}
            onSort={handleSort}
            onNavigate={handleNavigateToDirectory}
            onSelectCurrent={handleSelect}
            selectedIndex={selectedIndex}
            onSelectIndex={setSelectedIndex}
            canNavigateUp={canNavigateUp}
            onNavigateUp={navigateToParent}
          />
        )}
      </ModalContent>

      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button onClick={handleSelect} disabled={!currentPath || isLoading}>
          Select This Folder
        </Button>
      </ModalFooter>
    </Modal>
  )
}

export { FilesystemBrowser }
export type { FilesystemBrowserProps } from './FilesystemBrowser.types'
