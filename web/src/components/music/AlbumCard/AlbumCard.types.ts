import type { AlbumSummary } from '@/lib/types/music'

export interface AlbumCardProps {
  album: AlbumSummary
  onClick?: () => void
}
