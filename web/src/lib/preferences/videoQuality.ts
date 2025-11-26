/**
 * Video quality preferences - shared storage and types
 */

export interface VideoQualityPreferences {
  preferDataSaving: boolean
  preferQuality: boolean
  allowCellularHD: boolean
  allowCellular4K: boolean
  autoQualityEnabled: boolean
  maxAutoQuality: string | null
  minAutoQuality: string | null
  preferredCodec: string | null
}

export const DEFAULT_VIDEO_PREFERENCES: VideoQualityPreferences = {
  preferDataSaving: false,
  preferQuality: false,
  allowCellularHD: false,
  allowCellular4K: false,
  autoQualityEnabled: true,
  maxAutoQuality: null,
  minAutoQuality: null,
  preferredCodec: null,
}

const STORAGE_KEY = 'video_quality_preferences'

/**
 * Load preferences from localStorage
 */
export const loadVideoPreferences = (): VideoQualityPreferences => {
  if (typeof window === 'undefined') {
    return DEFAULT_VIDEO_PREFERENCES
  }

  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored) {
    try {
      return { ...DEFAULT_VIDEO_PREFERENCES, ...JSON.parse(stored) }
    } catch {
      return DEFAULT_VIDEO_PREFERENCES
    }
  }
  return DEFAULT_VIDEO_PREFERENCES
}

/**
 * Save preferences to localStorage
 */
export const saveVideoPreferences = (prefs: VideoQualityPreferences): void => {
  if (typeof window === 'undefined') {
    return
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs))
}
