import type { QualityOption } from '@/lib/hooks/useMediaPlayback'

export interface QualitySelectorProps {
  /** Available quality options from backend (filtered by source resolution) */
  availableQualities: QualityOption[]
  /** Currently selected quality ID */
  selectedQualityId: string | null
  /** Called when user selects a quality - triggers stream reload from current position */
  onQualityChange: (qualityId: string) => void
}
