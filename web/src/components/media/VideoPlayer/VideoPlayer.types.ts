export interface MediaMetadata {
  title: string
  subtitle?: string // For TV: "S01E02 - Episode Title", for Movies: year
  posterUrl?: string
}

export interface VideoPlayerProps {
  mediaId: number
  streamUrl: string
  initialPosition?: number // in seconds
  duration?: number // in seconds
  metadata?: MediaMetadata
  onClose?: () => void
}
