export { VideoPlayer } from './VideoPlayer'
export type { VideoPlayerProps, MediaMetadata } from './VideoPlayer.types'
export { QualitySelector } from './QualitySelector'
export { QualityPreferences } from './QualityPreferences'
export { NetworkOverlay } from './NetworkOverlay'
export { BufferIndicator } from './BufferIndicator'
export { NerdMenu } from './NerdMenu'
export { DropdownBase } from './DropdownBase'
export { DropdownSelector } from './DropdownSelector'
export type { DropdownOption } from './DropdownSelector'
export { SpeedSelector } from './SpeedSelector'

// Re-export preferences from shared module
export {
  loadVideoPreferences,
  saveVideoPreferences,
  DEFAULT_VIDEO_PREFERENCES,
} from '@/lib/preferences'
export type { VideoQualityPreferences } from '@/lib/preferences'
