export interface BreadcrumbNavProps {
  currentPath: string | null
  onNavigate: (path: string) => void
  onNavigateUp: () => void
  canNavigateUp: boolean
  isLoading: boolean
}
