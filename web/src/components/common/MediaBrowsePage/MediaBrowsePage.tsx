import { useState, useEffect, useRef, type ReactNode } from 'react'
import { Card, CardContent, Input } from '@/components/ui'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { SortSelector } from '@/components/common/SortSelector'
import { AdvancedFilters, type FilterState } from '@/components/common/AdvancedFilters'
import { ViewToggle, type ViewMode } from '@/components/common/ViewToggle'
import { useLibraryFilter, useDebounce, useGlobalKeyboardShortcuts, useWatchedList } from '@/lib/hooks'
import type { MediaBrowsePageProps } from './MediaBrowsePage.types'

/**
 * Reusable wrapper for media browsing pages (Movies, TV Shows, Music).
 * Provides common layout, filtering UI, loading states, and search functionality with debouncing.
 */
export const MediaBrowsePage = <T extends { id: number; title?: string; name?: string }>({
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
  renderListItem,
  getItemSearchText,

  // Interaction handlers
  onItemSelect,

  // URL state preservation
  onSearchChange,
  onSortChange,
  onFiltersChange,
  onViewModeChange,
  initialSearch = '',
  initialSort = 'title-asc',
  initialFilters = { genres: [], yearMin: undefined, yearMax: undefined, qualities: [], watchedFilter: 'all' },
  initialViewMode = 'grid',

  // Advanced filters configuration
  enableAdvancedFilters = false,
  genres = [],
  yearRange,
  qualityOptions = [],
  showWatchedFilter = false,
  getItemGenres,
  getItemYear,
  getItemQuality,

  // Optional overrides
  additionalFilters,
  customHeader,
  customEmpty,
  customGridRenderer,
}: MediaBrowsePageProps<T>): ReactNode => {
  const [searchQuery, setSearchQuery] = useState(initialSearch)
  const [sortBy, setSortBy] = useState(initialSort)
  const [filters, setFilters] = useState<FilterState>(initialFilters)
  const [viewMode, setViewMode] = useState<ViewMode>(initialViewMode)
  const [showHelpModal, setShowHelpModal] = useState(false)
  const searchInputRef = useRef<HTMLInputElement>(null)

  // Auto-enable view toggle if renderListItem is provided
  const enableViewToggle = !!renderListItem

  // Global keyboard shortcuts
  useGlobalKeyboardShortcuts({
    onSearch: () => searchInputRef.current?.focus(),
    onHelp: () => setShowHelpModal(true),
  })

  // Sync state with URL params when they change externally
  useEffect(() => {
    setSearchQuery(initialSearch)
  }, [initialSearch])

  useEffect(() => {
    setSortBy(initialSort)
  }, [initialSort])

  useEffect(() => {
    setFilters(initialFilters)
  }, [initialFilters])

  useEffect(() => {
    setViewMode(initialViewMode)
  }, [initialViewMode])

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

  useEffect(() => {
    if (onFiltersChange) {
      onFiltersChange(filters)
    }
  }, [filters, onFiltersChange])

  useEffect(() => {
    if (onViewModeChange) {
      onViewModeChange(viewMode)
    }
  }, [viewMode, onViewModeChange])

  // Fetch watched items for filtering
  const { data: watchedData } = useWatchedList({ limit: 10000 })
  const watchedMediaIds = new Set(watchedData?.progress?.map((p) => p.media_id) || [])

  // Filter items by debounced search query and advanced filters
  const filteredItems = data.filter((item) => {
    // Text search filter
    if (debouncedSearchQuery !== '') {
      const searchText = getItemSearchText ? getItemSearchText(item) : (item.title || item.name || '')
      if (!searchText.toLowerCase().includes(debouncedSearchQuery.toLowerCase())) {
        return false
      }
    }

    // Genre filter
    if (filters.genres && filters.genres.length > 0 && getItemGenres) {
      const itemGenres = getItemGenres(item) || []
      const hasMatchingGenre = filters.genres.some((filterGenre) =>
        itemGenres.some((itemGenre) => itemGenre.toLowerCase() === filterGenre.toLowerCase())
      )
      if (!hasMatchingGenre) {
        return false
      }
    }

    // Year range filter
    if ((filters.yearMin !== undefined || filters.yearMax !== undefined) && getItemYear) {
      const itemYear = getItemYear(item)
      if (itemYear !== undefined) {
        if (filters.yearMin !== undefined && itemYear < filters.yearMin) {
          return false
        }
        if (filters.yearMax !== undefined && itemYear > filters.yearMax) {
          return false
        }
      }
    }

    // Quality filter
    if (filters.qualities && filters.qualities.length > 0 && getItemQuality) {
      const itemQuality = getItemQuality(item)
      if (!itemQuality || !filters.qualities.includes(itemQuality)) {
        return false
      }
    }

    // Watched/unwatched filter
    if (filters.watchedFilter && filters.watchedFilter !== 'all') {
      const isWatched = watchedMediaIds.has(item.id)
      if (filters.watchedFilter === 'watched' && !isWatched) {
        return false
      }
      if (filters.watchedFilter === 'unwatched' && isWatched) {
        return false
      }
    }

    return true
  })

  // Client-side sorting (backend supports title_asc/title_desc, other sorts handled here)
  const sortedItems = [...filteredItems].sort((a, b) => {
    const [field, direction] = sortBy.split('-')

    let aVal: string | number
    let bVal: string | number

    switch (field) {
      case 'title':
        aVal = (a.title || a.name || '').toLowerCase()
        bVal = (b.title || b.name || '').toLowerCase()
        break
      case 'year':
        aVal = (a as T & { year?: number }).year || 0
        bVal = (b as T & { year?: number }).year || 0
        break
      case 'added':
        aVal = (a as T & { created_at?: string; date_added?: string }).created_at || (a as T & { created_at?: string; date_added?: string }).date_added || 0
        bVal = (b as T & { created_at?: string; date_added?: string }).created_at || (b as T & { created_at?: string; date_added?: string }).date_added || 0
        break
      case 'rating':
        aVal = (a as T & { rating?: number; imdb_rating?: number }).rating || (a as T & { rating?: number; imdb_rating?: number }).imdb_rating || 0
        bVal = (b as T & { rating?: number; imdb_rating?: number }).rating || (b as T & { rating?: number; imdb_rating?: number }).imdb_rating || 0
        break
      default:
        aVal = (a.title || a.name || '').toLowerCase()
        bVal = (b.title || b.name || '').toLowerCase()
    }

    if (aVal < bVal) {
      return direction === 'asc' ? -1 : 1
    }
    if (aVal > bVal) {
      return direction === 'asc' ? 1 : -1
    }
    return 0
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
            <div className={`grid gap-4 ${enableViewToggle ? 'grid-cols-1 md:grid-cols-3' : additionalFilters ? 'grid-cols-1 md:grid-cols-3' : 'grid-cols-1 md:grid-cols-2'}`}>
              <Input
                ref={searchInputRef}
                label="Search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={searchPlaceholder}
                aria-label={`Search ${type}`}
                aria-describedby="search-results-count"
                helperText="Press / or Cmd+K to focus"
              />
              <SortSelector
                value={sortBy}
                onChange={(newSort) => setSortBy(newSort)}
              />
              {enableViewToggle && (
                <div>
                  <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">
                    View
                  </label>
                  <ViewToggle value={viewMode} onChange={setViewMode} />
                </div>
              )}
              {additionalFilters}
            </div>

            {/* Advanced Filters */}
            {enableAdvancedFilters && (
              <div className="mt-4">
                <AdvancedFilters
                  genres={genres}
                  yearRange={yearRange}
                  qualityOptions={qualityOptions}
                  showWatchedFilter={showWatchedFilter}
                  value={filters}
                  onChange={setFilters}
                />
              </div>
            )}
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
      ) : viewMode === 'list' && renderListItem ? (
        <div className="space-y-2" role="list" aria-label={`${type} list`}>
          {sortedItems.map((item) => renderListItem(item, libraryId))}
        </div>
      ) : (
        // Virtual grid renderer (TanStack Virtual)
        customGridRenderer
      )}

      {/* Count display */}
      <div
        id="search-results-count"
        className="mt-4 text-sm text-neutral-500 dark:text-neutral-500 text-center"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        Showing {sortedItems.length} of {data.length} {type}
      </div>

      {/* Help Modal */}
      {showHelpModal && (
        <div
          className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
          onClick={() => setShowHelpModal(false)}
        >
          <div
            className="bg-white dark:bg-neutral-900 rounded-lg shadow-xl dark:shadow-neutral-950/50 max-w-md w-full mx-4 p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-lg font-semibold text-neutral-900 dark:text-neutral-50">Keyboard Shortcuts</h2>
              <button
                onClick={() => setShowHelpModal(false)}
                className="text-neutral-400 dark:text-neutral-600 hover:text-neutral-600 dark:hover:text-neutral-400 min-h-11 min-w-11 flex items-center justify-center"
                aria-label="Close help modal"
              >
                ✕
              </button>
            </div>
            <div className="space-y-3">
              <div className="flex justify-between items-center py-2 border-b border-neutral-200 dark:border-neutral-800">
                <span className="text-sm text-neutral-600 dark:text-neutral-400">Focus search</span>
                <kbd className="px-2 py-1 text-xs font-semibold bg-neutral-100 dark:bg-neutral-800 border border-neutral-300 dark:border-neutral-700 rounded text-neutral-900 dark:text-neutral-50">
                  / or Cmd+K
                </kbd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-neutral-200 dark:border-neutral-800">
                <span className="text-sm text-neutral-600 dark:text-neutral-400">Navigate grid</span>
                <kbd className="px-2 py-1 text-xs font-semibold bg-neutral-100 dark:bg-neutral-800 border border-neutral-300 dark:border-neutral-700 rounded text-neutral-900 dark:text-neutral-50">
                  Arrow keys
                </kbd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-neutral-200 dark:border-neutral-800">
                <span className="text-sm text-neutral-600 dark:text-neutral-400">Select item</span>
                <kbd className="px-2 py-1 text-xs font-semibold bg-neutral-100 dark:bg-neutral-800 border border-neutral-300 dark:border-neutral-700 rounded text-neutral-900 dark:text-neutral-50">
                  Enter
                </kbd>
              </div>
              <div className="flex justify-between items-center py-2">
                <span className="text-sm text-neutral-600 dark:text-neutral-400">Show shortcuts</span>
                <kbd className="px-2 py-1 text-xs font-semibold bg-neutral-100 dark:bg-neutral-800 border border-neutral-300 dark:border-neutral-700 rounded text-neutral-900 dark:text-neutral-50">
                  ?
                </kbd>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
