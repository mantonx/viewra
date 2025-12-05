import type { SubtitleTrack } from '@/lib/types/subtitles'

export interface SubtitleSelectorProps {
  availableSubtitles: SubtitleTrack[]
  currentSubtitle: number | null // null means "Off"
  onSubtitleChange: (trackId: number | null) => void
}
