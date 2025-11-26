/**
 * Hook for managing video quality preferences
 * Uses the shared preferences module for storage
 */
import { useState, useCallback } from 'react'
import {
  loadVideoPreferences,
  saveVideoPreferences,
  DEFAULT_VIDEO_PREFERENCES,
} from '@/lib/preferences'
import type { VideoQualityPreferences } from '@/lib/preferences'

// Re-export types for backwards compatibility
export type { VideoQualityPreferences }
export { DEFAULT_VIDEO_PREFERENCES }

/**
 * Hook for managing video quality preferences
 */
export const useQualityPreferences = () => {
  const [preferences, setPreferences] = useState<VideoQualityPreferences>(loadVideoPreferences)

  /**
   * Update preferences
   */
  const updatePreferences = useCallback((updates: Partial<VideoQualityPreferences>) => {
    setPreferences((prev) => {
      const updated = { ...prev, ...updates }

      // Ensure preferDataSaving and preferQuality are mutually exclusive
      if (updates.preferDataSaving && updated.preferQuality) {
        updated.preferQuality = false
      } else if (updates.preferQuality && updated.preferDataSaving) {
        updated.preferDataSaving = false
      }

      saveVideoPreferences(updated)
      return updated
    })
  }, [])

  /**
   * Toggle data saving mode
   */
  const toggleDataSaving = useCallback(() => {
    updatePreferences({ preferDataSaving: !preferences.preferDataSaving })
  }, [preferences.preferDataSaving, updatePreferences])

  /**
   * Toggle quality mode
   */
  const toggleQuality = useCallback(() => {
    updatePreferences({ preferQuality: !preferences.preferQuality })
  }, [preferences.preferQuality, updatePreferences])

  /**
   * Reset to defaults
   */
  const resetPreferences = useCallback(() => {
    setPreferences(DEFAULT_VIDEO_PREFERENCES)
    saveVideoPreferences(DEFAULT_VIDEO_PREFERENCES)
  }, [])

  return {
    preferences,
    updatePreferences,
    toggleDataSaving,
    toggleQuality,
    resetPreferences,
  }
}
