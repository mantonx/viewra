/**
 * Hook for managing video quality preferences
 * Stores user preferences for quality vs data saving trade-offs
 */
import { useState, useCallback } from 'react'

const STORAGE_KEY = 'viewra_quality_preferences'

export interface QualityPreferences {
  /** Prefer lower quality to save data/bandwidth */
  preferDataSaving: boolean
  /** Prefer higher quality over data savings */
  preferQuality: boolean
  /** Manually selected quality override (e.g., "1080p", "720p") */
  manualQuality: string | null
}

export const DEFAULT_QUALITY_PREFERENCES: QualityPreferences = {
  preferDataSaving: false,
  preferQuality: false,
  manualQuality: null,
}

/**
 * Load preferences from localStorage
 */
const loadPreferences = (): QualityPreferences => {
  if (typeof window === 'undefined') {
    return DEFAULT_QUALITY_PREFERENCES
  }

  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      return {
        ...DEFAULT_QUALITY_PREFERENCES,
        ...parsed,
      }
    }
  } catch (error) {
    console.error('Failed to load quality preferences:', error)
  }

  return DEFAULT_QUALITY_PREFERENCES
}

/**
 * Save preferences to localStorage
 */
const savePreferences = (preferences: QualityPreferences): void => {
  if (typeof window === 'undefined') {
    return
  }

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences))
  } catch (error) {
    console.error('Failed to save quality preferences:', error)
  }
}

/**
 * Hook for managing video quality preferences
 */
export const useQualityPreferences = () => {
  const [preferences, setPreferences] = useState<QualityPreferences>(loadPreferences)

  /**
   * Update preferences
   */
  const updatePreferences = useCallback((updates: Partial<QualityPreferences>) => {
    setPreferences((prev) => {
      const updated = { ...prev, ...updates }

      // Ensure preferDataSaving and preferQuality are mutually exclusive
      if (updates.preferDataSaving && updated.preferQuality) {
        updated.preferQuality = false
      } else if (updates.preferQuality && updated.preferDataSaving) {
        updated.preferDataSaving = false
      }

      savePreferences(updated)
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
   * Set manual quality override
   */
  const setManualQuality = useCallback(
    (quality: string | null) => {
      updatePreferences({ manualQuality: quality })
    },
    [updatePreferences]
  )

  /**
   * Reset to defaults
   */
  const resetPreferences = useCallback(() => {
    setPreferences(DEFAULT_QUALITY_PREFERENCES)
    savePreferences(DEFAULT_QUALITY_PREFERENCES)
  }, [])

  return {
    preferences,
    updatePreferences,
    toggleDataSaving,
    toggleQuality,
    setManualQuality,
    resetPreferences,
  }
}
