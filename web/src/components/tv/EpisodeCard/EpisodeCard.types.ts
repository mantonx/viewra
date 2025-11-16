import type { TVEpisodeResponse } from '@/lib/types/tv'

export interface EpisodeCardProps {
  episode: TVEpisodeResponse
  onClick?: () => void
}
