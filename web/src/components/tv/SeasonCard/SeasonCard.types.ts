import type { SeasonGroup } from '@/lib/types/tv'

export interface SeasonCardProps {
  season: SeasonGroup
  showTitle: string
  onClick?: () => void
}
