import type { VideoQualityPreferences } from '@/lib/preferences'

export interface QualityPreferencesProps {
  isOpen: boolean
  onClose: () => void
  onPreferencesChange?: (prefs: VideoQualityPreferences) => void
}
