import type { TVShowSummary } from '@/lib/types/tv'

export interface TVShowCardProps {
  show: TVShowSummary
  onClick?: () => void
}
