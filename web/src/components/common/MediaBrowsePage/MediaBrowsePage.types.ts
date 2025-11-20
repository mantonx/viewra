import type { ReactNode } from 'react'
import type { LibraryType } from '@/lib/hooks'

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
  getItemSearchText?: (item: T) => string
  gridClassName?: string

  // Interaction handlers
  onItemSelect?: (item: T) => void

  // URL state preservation
  onSearchChange?: (search: string) => void
  onSortChange?: (sort: string) => void
  initialSearch?: string
  initialSort?: string

  // Optional customizations
  additionalFilters?: ReactNode
  customHeader?: ReactNode
  customEmpty?: ReactNode
}
