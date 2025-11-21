import type { ReactNode } from 'react'
import type { ViewMode } from '../ViewToggle'

export interface MediaItem {
  id: number
  title?: string
  name?: string
}

export interface InfiniteMediaGridProps<T extends MediaItem> {
  // Data source
  type: 'movies' | 'tv' | 'music'
  libraryId: number
  sort?: string

  // Rendering
  renderCard: (item: T) => ReactNode
  renderListItem?: (item: T) => ReactNode

  // Filtering (applied client-side after loading)
  searchQuery?: string
  genres?: string[]
  yearMin?: number
  yearMax?: number
  qualities?: string[]
  watchedFilter?: 'all' | 'watched' | 'unwatched'

  // View mode
  viewMode?: ViewMode
  gridClassName?: string

  // Callbacks
  onItemSelect?: (item: T) => void

  // Customization
  emptyState?: ReactNode
  loadingIndicator?: ReactNode

  // Helper functions for filtering
  getItemSearchText?: (item: T) => string
  getItemGenres?: (item: T) => string[] | undefined
  getItemYear?: (item: T) => number | undefined
  getItemQuality?: (item: T) => string | undefined
}
