import type { MusicTrackResponse } from '@/lib/types/music'

export interface TrackListProps {
  tracks: MusicTrackResponse[]
  currentTrackId?: number | null
  onTrackClick?: (track: MusicTrackResponse) => void
  onPlayAll?: () => void
}
