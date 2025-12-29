// Main component
export { VideoPlayer } from './VideoPlayer'
export type { VideoPlayerProps } from './VideoPlayer.types'

// Control components
export { VideoControls } from './VideoControls'
export { QualitySelector } from './QualitySelector'
export { QualityPreferences } from './QualityPreferences'
export { AudioSelector } from './AudioSelector'
export { SubtitleSelector } from './SubtitleSelector'
export { SpeedSelector } from './SpeedSelector'
export { NerdMenu } from './NerdMenu'

// Overlay components
export { SubtitleOverlay } from './SubtitleOverlay'
export { BufferIndicator } from './BufferIndicator'
export { StatsPanel } from './StatsPanel'

// Base components
export { DropdownBase } from './DropdownBase'
export { DropdownSelector } from './DropdownSelector'
export type { DropdownOption } from './DropdownSelector'

// Re-export shared types from lib
export type { MediaMetadata, AudioTrack, QualityOption } from '@/lib/types/video'
export type { SubtitleTrack } from '@/lib/types/subtitles'

// Re-export preferences from shared module
export {
  loadVideoPreferences,
  saveVideoPreferences,
  DEFAULT_VIDEO_PREFERENCES,
} from '@/lib/preferences'
export type { VideoQualityPreferences } from '@/lib/preferences'
