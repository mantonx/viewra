import type { ReactNode } from 'react'
import type { LibraryType } from '@/lib/hooks'
import type { FilterState, YearRange } from '../AdvancedFilters'
import type { ViewMode } from '../ViewToggle'

export interface MediaBrowsePageProps<T extends { id: number; title?: string; name?: string }> {
  // Page configuration
  type: LibraryType
  title: string
  description: string
  searchPlaceholder: string
  emptyIcon: string
  emptyTitle: string
  emptyDescription: string

  // Data
  data: T[]
  isLoading: boolean
  error: Error | null

  // Item rendering
  renderItem: (item: T, libraryId: number) => ReactNode
  renderListItem?: (item: T, libraryId: number) => ReactNode
  getItemSearchText?: (item: T) => string

  // Interaction handlers
  onItemSelect?: (item: T) => void

  // URL state preservation
  onSearchChange?: (search: string) => void
  onSortChange?: (sort: string) => void
  onFiltersChange?: (filters: FilterState) => void
  onViewModeChange?: (viewMode: ViewMode) => void
  initialSearch?: string
  initialSort?: string
  initialFilters?: FilterState
  initialViewMode?: ViewMode

  // Advanced filters configuration
  enableAdvancedFilters?: boolean
  genres?: string[]
  yearRange?: YearRange
  qualityOptions?: string[]
  showWatchedFilter?: boolean
  getItemGenres?: (item: T) => string[] | undefined
  getItemYear?: (item: T) => number | undefined
  getItemQuality?: (item: T) => string | undefined

  // Optional customizations
  additionalFilters?: ReactNode
  customHeader?: ReactNode
  customEmpty?: ReactNode
  customGridRenderer?: ReactNode // For virtual scrolling
}
