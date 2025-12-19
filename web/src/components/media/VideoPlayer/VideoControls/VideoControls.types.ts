import type { AudioTrack, MediaMetadata } from '@/lib/types/video'
import type { SubtitleTrack } from '@/lib/types/subtitles'
import type { QualityOption } from '@/lib/hooks/useMediaPlayback'

export interface VideoControlsProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  isPlaying: boolean
  currentTime: number
  duration: number
  volume: number
  isMuted: boolean
  isFullscreen: boolean
  isPiP: boolean
  /** Available quality options from backend (filtered by source resolution) */
  availableQualities: QualityOption[]
  /** Currently selected quality ID */
  selectedQualityId: string | null
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
  /** Called when user selects a quality - triggers stream reload from current position */
  onQualityChange: (qualityId: string) => void
  onAudioTrackChange: (trackId: number) => void
  onSubtitleChange: (trackId: number | null) => void
  onSpeedChange: (speed: number) => void
  onSkip: (seconds: number) => void
  onToggleStats: () => void
}
