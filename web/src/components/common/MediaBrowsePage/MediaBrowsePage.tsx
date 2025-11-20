import { useState, useEffect, type ReactNode } from 'react'
import { Card, CardContent, Input, Select } from '@/components/ui'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { useLibraryFilter, useDebounce, useGridNavigation } from '@/lib/hooks'
import type { MediaBrowsePageProps } from './MediaBrowsePage.types'

/**
 * Reusable wrapper for media browsing pages (Movies, TV Shows, Music).
 * Provides common layout, filtering UI, loading states, and search functionality with debouncing.
 */
export function MediaBrowsePage<T extends { id: number; title?: string; name?: string }>({
  // Page configuration
  type,
  title,
  description,
  searchPlaceholder,
  emptyIcon,
  emptyTitle,
  emptyDescription,

  // Data
  data,
  isLoading,
  error,

  // Item rendering
  renderItem,
  getItemSearchText,
  gridClassName = 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4',

  // Interaction handlers
  onItemSelect,

  // URL state preservation
  onSearchChange,
  onSortChange,
  initialSearch = '',
  initialSort = 'title-asc',

  // Optional overrides
  additionalFilters,
  customHeader,
  customEmpty,
}: MediaBrowsePageProps<T>): ReactNode {
  const [searchQuery, setSearchQuery] = useState(initialSearch)
  const [sortBy, setSortBy] = useState(initialSort)
  const [columns, setColumns] = useState(6)

  // Sync state with URL params when they change externally
  useEffect(() => {
    setSearchQuery(initialSearch)
  }, [initialSearch])

  useEffect(() => {
    setSortBy(initialSort)
  }, [initialSort])

  // Debounce search query to avoid filtering on every keystroke (300ms delay)
  const debouncedSearchQuery = useDebounce(searchQuery, 300)

  // Get libraryId for rendering (library filtering happens at the page level via useInfiniteMovies/etc hooks)
  const { libraryId } = useLibraryFilter(type)

  // Notify parent of state changes for URL preservation
  useEffect(() => {
    if (onSearchChange) {
      onSearchChange(searchQuery)
    }
  }, [searchQuery, onSearchChange])

  useEffect(() => {
    if (onSortChange) {
      onSortChange(sortBy)
    }
  }, [sortBy, onSortChange])

  // Calculate grid columns based on window width for keyboard navigation
  useEffect(() => {
    const updateColumns = () => {
      const width = window.innerWidth
      if (width >= 1280) setColumns(6) // xl
      else if (width >= 1024) setColumns(5) // lg
      else if (width >= 768) setColumns(4) // md
      else if (width >= 640) setColumns(3) // sm
      else setColumns(2) // base
    }

    updateColumns()
    window.addEventListener('resize', updateColumns)
    return () => window.removeEventListener('resize', updateColumns)
  }, [])

  // Filter items by debounced search query
  const filteredItems = data.filter((item) => {
    if (debouncedSearchQuery === '') return true
    const searchText = getItemSearchText ? getItemSearchText(item) : (item.title || item.name || '')
    return searchText.toLowerCase().includes(debouncedSearchQuery.toLowerCase())
  })

  // Sorting is now handled by the backend API
  const sortedItems = filteredItems


  // Use default grid class
  const actualGridClassName = gridClassName || 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4'

  // Keyboard navigation for media grid
  const gridRef = useGridNavigation({
    columns,
    itemCount: sortedItems.length,
    enabled: sortedItems.length > 0,
    onItemSelect: onItemSelect ? (index) => onItemSelect(sortedItems[index]) : undefined,
  })

  // Loading and error states
  if (isLoading) {
    return <LoadingPage text={`Loading ${type}...`} />
  }

  if (error) {
    return <ErrorPage error={error} context={type} />
  }

  return (
    <div className="p-8">
      {/* Page header */}
      {customHeader || <PageHeader title={title} description={description} />}

      {/* Filters */}
      <Card className="mb-6">
        <CardContent>
          <form role="search" aria-label={`Filter and sort ${type}`} onSubmit={(e) => e.preventDefault()}>
            <div className={`grid gap-4 ${additionalFilters ? 'grid-cols-1 md:grid-cols-3' : 'grid-cols-1 md:grid-cols-2'}`}>
              <Input
                label="Search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={searchPlaceholder}
                aria-label={`Search ${type}`}
                aria-describedby="search-results-count"
              />
              <Select
                label="Sort By"
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value)}
                options={[
                  { value: 'title-asc', label: 'Title (A-Z)' },
                  { value: 'title-desc', label: 'Title (Z-A)' },
                ]}
                aria-label={`Sort ${type} by`}
              />
              {additionalFilters}
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Items grid or empty state */}
      {sortedItems.length === 0 ? (
        customEmpty || (
          <Card>
            <CardContent>
              <EmptyState
                icon={emptyIcon}
                title={data.length === 0 ? emptyTitle : 'No matches'}
                description={
                  data.length === 0
                    ? emptyDescription
                    : `No ${type} match your search. Try adjusting your query.`
                }
              />
            </CardContent>
          </Card>
        )
      ) : (
        <div ref={gridRef} className={actualGridClassName} role="grid" aria-label={`${type} grid`}>
          {sortedItems.map((item) => renderItem(item, libraryId))}
        </div>
      )}

      {/* Count display */}
      <div
        id="search-results-count"
        className="mt-4 text-sm text-gray-500 text-center"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        Showing {sortedItems.length} of {data.length} {type}
      </div>
    </div>
  )
}
