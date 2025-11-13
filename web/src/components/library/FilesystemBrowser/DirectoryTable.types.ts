import type { SortBy, SortOrder } from '@/lib/hooks/useBrowserPreferences'

export interface Directory {
  name?: string
  path?: string
  readable?: boolean
  modified_at?: string
}

export interface DirectoryTableProps {
  directories: Directory[]
  isLoading: boolean
  sortBy: SortBy
  sortOrder: SortOrder
  onSort: (column: SortBy) => void
  onNavigate: (path: string) => void
  onSelectCurrent: () => void
  selectedIndex: number
  onSelectIndex: (index: number) => void
  canNavigateUp: boolean
  onNavigateUp: () => void
}
