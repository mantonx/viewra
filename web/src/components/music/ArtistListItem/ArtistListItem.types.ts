import type { ArtistSummary } from '@/lib/types/music'

export interface ArtistListItemProps {
  artist: ArtistSummary
  onClick?: () => void
}
