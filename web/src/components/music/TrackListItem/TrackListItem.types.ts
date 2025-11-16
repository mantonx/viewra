import type { MusicTrackResponse } from '@/lib/types/music'

export interface TrackListItemProps {
  track: MusicTrackResponse
  isPlaying?: boolean
  onClick?: () => void
}
