import type { BadgePreferences } from '@/lib/types/preferences'

export interface MediaBadgesProps {
  preferences: BadgePreferences
  badges: {
    isNew?: boolean
    isExtra?: boolean
    resolution?: string
    contentRating?: string
    codec?: string
    mediaType?: string
  }
}
