import type { TVShowSummary } from '@/lib/types/tv'

export interface TVShowListItemProps {
  show: TVShowSummary
  onClick?: () => void
}
