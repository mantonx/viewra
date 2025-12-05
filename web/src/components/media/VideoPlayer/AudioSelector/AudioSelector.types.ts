import type { AudioTrack } from '@/lib/types/video'

export interface AudioSelectorProps {
  availableAudioTracks: AudioTrack[]
  currentAudioTrack: number
  onAudioTrackChange: (trackId: number) => void
}
