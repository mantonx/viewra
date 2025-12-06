import type { AudioTrack, MediaMetadata } from '@/lib/types/video'
import type { SubtitleTrack } from '@/lib/types/subtitles'

export interface VideoControlsProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  isPlaying: boolean
  currentTime: number
  duration: number
  volume: number
  isMuted: boolean
  isFullscreen: boolean
  isPiP: boolean
  availableQualities: Array<{ height: number; bandwidth: number }>
  currentQuality: number | null
  currentBandwidth?: number | null
  autoMode: boolean
  availableAudioTracks: AudioTrack[]
  currentAudioTrack: number
  availableSubtitles: SubtitleTrack[]
  currentSubtitle: number | null
  playbackSpeed: number
  metadata?: MediaMetadata
  showStats: boolean
  onPlayPause: () => void
  onSeek: (time: number) => void
  onVolumeChange: (volume: number) => void
  onMuteToggle: () => void
  onFullscreenToggle: () => void
  onPiPToggle: () => void
  onQualityChange: (height: number, bandwidth?: number) => void
  onAutoToggle: () => void
  onAudioTrackChange: (trackId: number) => void
  onSubtitleChange: (trackId: number | null) => void
  onSpeedChange: (speed: number) => void
  onSkip: (seconds: number) => void
  onToggleStats: () => void
}
