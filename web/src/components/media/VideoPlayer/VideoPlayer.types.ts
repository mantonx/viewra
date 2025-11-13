export interface VideoPlayerProps {
  mediaId: number
  streamUrl: string
  initialPosition?: number // in seconds
  duration?: number // in seconds
  onClose?: () => void
}
