import type { ArtistSummary } from '@/lib/types/music'

export interface ArtistCardProps {
  artist: ArtistSummary
  onClick?: () => void
}
