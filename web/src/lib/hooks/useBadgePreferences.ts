import { useState, useEffect } from 'react'
import type { BadgePreferences } from '@/lib/types/preferences'
import { DEFAULT_BADGE_PREFERENCES } from '@/lib/types/preferences'

const STORAGE_KEY = 'viewra_badge_preferences'

export const useBadgePreferences = () => {
  const [preferences, setPreferences] = useState<BadgePreferences>(() => {
    // Load from localStorage on mount
    if (typeof window === 'undefined') return DEFAULT_BADGE_PREFERENCES

    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        return DEFAULT_BADGE_PREFERENCES
      }
    }
    return DEFAULT_BADGE_PREFERENCES
  })

  const updatePreferences = (newPreferences: BadgePreferences) => {
    setPreferences(newPreferences)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newPreferences))
  }

  return { preferences, updatePreferences }
}
