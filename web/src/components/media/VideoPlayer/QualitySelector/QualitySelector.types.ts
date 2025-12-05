import type { QualityOption } from '@/lib/types/video'
import type { QualityRecommendationResponse } from '@/lib/api/adaptive'

export interface QualitySelectorProps {
  currentQuality: number | null
  currentBandwidth?: number | null
  availableQualities: QualityOption[]
  recommendedQuality: QualityRecommendationResponse | null
  autoMode: boolean
  onQualityChange: (height: number, bandwidth?: number) => void
  onAutoToggle: () => void
  showBitrateVariants?: boolean
}
