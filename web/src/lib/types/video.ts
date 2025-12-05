/**
 * Shared video player types used across the application.
 */

export interface MediaMetadata {
  title: string
  subtitle?: string // For TV: "S01E02 - Episode Title", for Movies: year
  posterUrl?: string
}

export interface AudioTrack {
  id: number
  name: string
  language: string
  isDefault?: boolean
  codec?: string
  channels?: number
}

export interface QualityOption {
  height: number
  bandwidth: number
  displayName?: string
  dataUsageMBPerHour?: number
  canDirectPlay?: boolean
  needsTranscode?: boolean
  isRecommended?: boolean
  isOriginal?: boolean // True if this is the source/original quality
  description?: string
  index?: number
}
