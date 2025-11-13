export interface DirectorySearchProps {
  searchQuery: string
  onSearchChange: (query: string) => void
  resultsCount: number
  totalCount: number
}
