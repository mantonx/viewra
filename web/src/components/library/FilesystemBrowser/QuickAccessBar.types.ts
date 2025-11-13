export interface QuickAccessBarProps {
  onNavigate: (path: string) => void
  recentPaths: string[]
  onClearRecent: () => void
  isLoading: boolean
}
