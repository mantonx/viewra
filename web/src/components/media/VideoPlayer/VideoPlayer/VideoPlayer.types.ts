import type { MediaMetadata } from '@/lib/types/video'

export interface VideoPlayerProps {
  mediaId: number
  streamUrl: string
  initialPosition?: number // in seconds
  duration?: number // in seconds
  metadata?: MediaMetadata
  onClose?: () => void
  onTimeUpdate?: (time: number) => void // Called periodically with current playback position
}
