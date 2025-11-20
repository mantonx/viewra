/**
 * Types for AdvancedFilters component
 */

export interface FilterState {
  genres?: string[]
  yearMin?: number
  yearMax?: number
  qualities?: string[]
  watchedFilter?: 'all' | 'watched' | 'unwatched'
}

export interface YearRange {
  min: number
  max: number
}

export interface AdvancedFiltersProps {
  /** Available genres to filter by */
  genres?: string[]

  /** Year range for filtering (e.g., {min: 1990, max: 2024}) */
  yearRange?: YearRange

  /** Available quality options (e.g., ['4K', '1080p', '720p']) */
  qualityOptions?: string[]

  /** Whether to show the watched/unwatched filter */
  showWatchedFilter?: boolean

  /** Current filter state */
  value: FilterState

  /** Callback when filters change */
  onChange: (newFilters: FilterState) => void
}
