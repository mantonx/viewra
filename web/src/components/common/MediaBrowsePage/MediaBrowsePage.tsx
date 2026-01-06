import { useState, useEffect, useRef, type ReactNode, cloneElement, isValidElement } from 'react'
import { Card, CardContent, Input } from '@/components/ui'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { SortSelector } from '@/components/common/SortSelector'
import { AdvancedFilters, type FilterState } from '@/components/common/AdvancedFilters'
import { ViewToggle, type ViewMode } from '@/components/common/ViewToggle'
import { useLibraryFilter, useDebounce, useGlobalKeyboardShortcuts, useWatchedList } from '@/lib/hooks'
import { useTheme } from '@/contexts'
import { bg, text, border, glassStyles } from '@/styles/semantic'
import { cn } from '@/lib/utils'
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
  renderItem: _renderItem,
  renderListItem,
  getItemSearchText,

  // Interaction handlers
  onItemSelect: _onItemSelect,

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

  // Server-side search flag
  serverSideSearch = false,

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
  const [isHeaderMinimized, setIsHeaderMinimized] = useState(false)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const _scrollContainerRef = useRef<HTMLDivElement>(null)
  const { theme } = useTheme()

  // Auto-enable view toggle if renderListItem is provided
  const enableViewToggle = !!renderListItem

  // Global keyboard shortcuts
  useGlobalKeyboardShortcuts({
    onSearch: () => searchInputRef.current?.focus(),
    onHelp: () => setShowHelpModal(true),
  })

  // Refs to track if we've initialized state (to avoid loops when callbacks update URL)
  const isInitializedRef = useRef(false)

  // Refs to track the last synced initial values (to detect external URL changes)
  const lastSyncedSearchRef = useRef(initialSearch)
  const lastSyncedSortRef = useRef(initialSort)
  const lastSyncedFiltersRef = useRef(JSON.stringify(initialFilters))
  const lastSyncedViewModeRef = useRef(initialViewMode)

  // Sync state with URL params when they change externally (only after initial mount)
  useEffect(() => {
    if (isInitializedRef.current && initialSearch !== lastSyncedSearchRef.current) {
      lastSyncedSearchRef.current = initialSearch
      setSearchQuery(initialSearch)
    }
  }, [initialSearch])

  useEffect(() => {
    if (isInitializedRef.current && initialSort !== lastSyncedSortRef.current) {
      lastSyncedSortRef.current = initialSort
      setSortBy(initialSort)
    }
  }, [initialSort])

  useEffect(() => {
    const initialFiltersStr = JSON.stringify(initialFilters)
    if (isInitializedRef.current && initialFiltersStr !== lastSyncedFiltersRef.current) {
      lastSyncedFiltersRef.current = initialFiltersStr
      setFilters(initialFilters)
    }
  }, [initialFilters])

  useEffect(() => {
    if (isInitializedRef.current && initialViewMode !== lastSyncedViewModeRef.current) {
      lastSyncedViewModeRef.current = initialViewMode
      setViewMode(initialViewMode)
    }
  }, [initialViewMode])

  // Mark as initialized after first render
  useEffect(() => {
    isInitializedRef.current = true
  }, [])

  // Debounce search query to avoid filtering on every keystroke (300ms delay)
  const debouncedSearchQuery = useDebounce(searchQuery, 300)

  // Get libraryId for rendering (library filtering happens at the page level via useInfiniteMovies/etc hooks)
  const { libraryId } = useLibraryFilter(type)

  // Use refs to track previous values to detect actual user changes vs prop syncing
  const prevSearchRef = useRef(searchQuery)
  const prevSortRef = useRef(sortBy)
  const prevFiltersRef = useRef(filters)
  const prevViewModeRef = useRef(viewMode)

  // Notify parent of state changes for URL preservation (only when user changes state, not from prop sync)
  useEffect(() => {
    if (isInitializedRef.current && onSearchChange && searchQuery !== prevSearchRef.current) {
      prevSearchRef.current = searchQuery
      lastSyncedSearchRef.current = searchQuery // Update last synced to match what we're about to send to URL
      onSearchChange(searchQuery)
    }
  }, [searchQuery, onSearchChange])

  useEffect(() => {
    if (isInitializedRef.current && onSortChange && sortBy !== prevSortRef.current) {
      prevSortRef.current = sortBy
      lastSyncedSortRef.current = sortBy // Update last synced to match what we're about to send to URL
      onSortChange(sortBy)
    }
  }, [sortBy, onSortChange])

  useEffect(() => {
    if (isInitializedRef.current && onFiltersChange) {
      const filtersChanged = JSON.stringify(filters) !== JSON.stringify(prevFiltersRef.current)
      if (filtersChanged) {
        prevFiltersRef.current = filters
        lastSyncedFiltersRef.current = JSON.stringify(filters) // Update last synced to match what we're about to send to URL
        onFiltersChange(filters)
      }
    }
  }, [filters, onFiltersChange])

  useEffect(() => {
    if (isInitializedRef.current && onViewModeChange && viewMode !== prevViewModeRef.current) {
      prevViewModeRef.current = viewMode
      lastSyncedViewModeRef.current = viewMode // Update last synced to match what we're about to send to URL
      onViewModeChange(viewMode)
    }
  }, [viewMode, onViewModeChange])

  // Fetch watched items for filtering
  const { data: watchedData } = useWatchedList({ limit: 10000 })
  const watchedMediaIds = new Set(watchedData?.progress?.map((p) => p.media_id) || [])

  // Filter items by debounced search query and advanced filters
  const filteredItems = data.filter((item) => {
    // Text search filter (skip if serverSideSearch is enabled - data is already filtered)
    if (!serverSideSearch && debouncedSearchQuery !== '') {
      const searchText = getItemSearchText ? getItemSearchText(item) : (item.title || item.name || '')
      const matches = searchText.toLowerCase().includes(debouncedSearchQuery.toLowerCase())
      if (!matches) {
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
  // Skip sorting when there's an active search with serverSideSearch enabled,
  // because semantic search returns results ordered by relevance.
  // Check multiple sources (searchQuery, initialSearch, debouncedSearchQuery) to handle timing edge cases.
  const hasActiveSearch = searchQuery !== '' || initialSearch !== '' || debouncedSearchQuery !== ''
  const sortedItems = (serverSideSearch && hasActiveSearch)
    ? filteredItems
    : [...filteredItems].sort((a, b) => {
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
    <div className="h-full overflow-hidden flex flex-col relative">
      <div className={`sticky top-0 z-10 flex-shrink-0 transition-all duration-300 ease-in-out ${isHeaderMinimized ? 'py-2 px-4' : 'p-4 pb-0'}`} style={{ background: 'transparent' }}>
      {/* Page header */}
      <div className={`transition-all duration-300 ease-in-out overflow-hidden ${isHeaderMinimized ? 'max-h-0 opacity-0' : 'max-h-32 opacity-100'}`}>
        {customHeader || <PageHeader title={title} description={description} />}
      </div>

      {/* Filters */}
      <div
        className={`${isHeaderMinimized ? 'mb-3 px-3 py-2.5' : 'mb-6 px-5 py-4'} rounded-xl transition-all duration-300 ease-out border`}
        style={glassStyles.enhanced(theme === 'dark')}
      >
        <div>
          <form role="search" aria-label={`Filter and sort ${type}`} onSubmit={(e) => e.preventDefault()}>
            <div className={`grid gap-3 ${enableViewToggle ? 'grid-cols-1 md:grid-cols-3' : additionalFilters ? 'grid-cols-1 md:grid-cols-3' : 'grid-cols-1 md:grid-cols-2'}`}>
              <Input
                ref={searchInputRef}
                label={isHeaderMinimized ? undefined : "Search"}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={searchPlaceholder}
                aria-label={`Search ${type}`}
                aria-describedby="search-results-count"
                helperText={isHeaderMinimized ? undefined : "Press / or Cmd+K to focus"}
                className={`backdrop-blur-sm !transition-colors ${
                  theme === 'dark'
                    ? '!bg-white/5 !text-white placeholder:!text-white/50 !border-white/20 hover:!border-white/30 focus:!border-white/40'
                    : '!bg-black/5 !text-neutral-900 placeholder:!text-neutral-500 !border-black/20 hover:!border-black/30 focus:!border-black/40'
                }`}
              />
              <SortSelector
                value={sortBy}
                onChange={(newSort) => setSortBy(newSort)}
                showLabel={!isHeaderMinimized}
                className={`[&_button]:backdrop-blur-sm [&_button]:!transition-colors ${
                  theme === 'dark'
                    ? '[&_button]:!bg-white/5 [&_button]:!text-white [&_button]:!border-white/20 [&_button]:hover:!border-white/30'
                    : '[&_button]:!bg-black/5 [&_button]:!text-neutral-900 [&_button]:!border-black/20 [&_button]:hover:!border-black/30'
                }`}
              />
              {enableViewToggle && (
                <div>
                  <div className={`transition-all duration-300 ease-in-out overflow-hidden ${isHeaderMinimized ? 'max-h-0 opacity-0' : 'max-h-10 opacity-100'}`}>
                    <label className={cn('block text-sm font-medium mb-2', text.secondary)}>
                      View
                    </label>
                  </div>
                  <ViewToggle value={viewMode} onChange={setViewMode} />
                </div>
              )}
              {additionalFilters}
            </div>

            {/* Advanced Filters */}
            <div className={`transition-all duration-300 ease-in-out overflow-hidden ${enableAdvancedFilters && !isHeaderMinimized ? 'max-h-96 opacity-100 mt-4' : 'max-h-0 opacity-0'}`}>
              {enableAdvancedFilters && (
                <AdvancedFilters
                  genres={genres}
                  yearRange={yearRange}
                  qualityOptions={qualityOptions}
                  showWatchedFilter={showWatchedFilter}
                  value={filters}
                  onChange={setFilters}
                />
              )}
            </div>
          </form>
        </div>
      </div>
      </div>

      {/* Items grid or empty state */}
      <div
        className={`flex-1 min-h-0 ${viewMode === 'list' || sortedItems.length === 0 ? 'overflow-auto px-4 pb-32' : 'px-2'}`}
        onScroll={(e) => {
          // Track scroll for header minimization in list view and empty states
          if (viewMode === 'list' || sortedItems.length === 0) {
            setIsHeaderMinimized(e.currentTarget.scrollTop > 100)
          }
        }}
        style={{ marginTop: '-3rem' }}
      >
        {sortedItems.length === 0 ? (
          customEmpty || (
            <Card className="mt-20">
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
          <div className="space-y-2 pt-20" role="list" aria-label={`${type} list`}>
            {sortedItems.map((item) => renderListItem(item, libraryId))}
          </div>
        ) : (
          // Virtual grid renderer (TanStack Virtual handles its own scroll)
          // Pass filtered/sorted items to the custom grid renderer
          isValidElement(customGridRenderer)
            ? cloneElement(customGridRenderer as React.ReactElement<{ items?: T[]; onScroll?: (scrollTop: number) => void }>, {
                items: sortedItems,
                onScroll: (scrollTop: number) => {
                  setIsHeaderMinimized(scrollTop > 100)
                },
              })
            : customGridRenderer
        )}
      </div>

      {/* Help Modal */}
      {showHelpModal && (
        <div
          className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
          onClick={() => setShowHelpModal(false)}
        >
          <div
            className={cn('rounded-lg shadow-xl max-w-md w-full mx-4 p-6', bg.elevated, 'dark:shadow-neutral-950/50')}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-between items-center mb-4">
              <h2 className={cn('text-lg font-semibold', text.primary)}>Keyboard Shortcuts</h2>
              <button
                onClick={() => setShowHelpModal(false)}
                className={cn(
                  'min-h-11 min-w-11 flex items-center justify-center',
                  text.tertiary,
                  'hover:text-neutral-600 dark:hover:text-neutral-400'
                )}
                aria-label="Close help modal"
              >
                ✕
              </button>
            </div>
            <div className="space-y-3">
              <div className={cn('flex justify-between items-center py-2 border-b', border.secondary)}>
                <span className={cn('text-sm', text.secondary)}>Focus search</span>
                <kbd className={cn('px-2 py-1 text-xs font-semibold border rounded', bg.tertiary, border.secondary, text.primary)}>
                  / or Cmd+K
                </kbd>
              </div>
              <div className={cn('flex justify-between items-center py-2 border-b', border.secondary)}>
                <span className={cn('text-sm', text.secondary)}>Navigate grid</span>
                <kbd className={cn('px-2 py-1 text-xs font-semibold border rounded', bg.tertiary, border.secondary, text.primary)}>
                  Arrow keys
                </kbd>
              </div>
              <div className={cn('flex justify-between items-center py-2 border-b', border.secondary)}>
                <span className={cn('text-sm', text.secondary)}>Select item</span>
                <kbd className={cn('px-2 py-1 text-xs font-semibold border rounded', bg.tertiary, border.secondary, text.primary)}>
                  Enter
                </kbd>
              </div>
              <div className="flex justify-between items-center py-2">
                <span className={cn('text-sm', text.secondary)}>Show shortcuts</span>
                <kbd className={cn('px-2 py-1 text-xs font-semibold border rounded', bg.tertiary, border.secondary, text.primary)}>
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
