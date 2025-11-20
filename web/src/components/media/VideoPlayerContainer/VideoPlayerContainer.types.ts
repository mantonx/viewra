export interface PlaybackState {
  isPlaying: boolean
  mediaId: number | null
  streamUrl: string | null
  initialPosition: number
  transcodeState: 'idle' | 'checking' | 'transcoding' | 'ready' | 'direct'
}
