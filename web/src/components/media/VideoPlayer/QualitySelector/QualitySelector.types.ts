import type { QualityOption } from '@/lib/types/video'

export interface QualitySelectorProps {
  currentQuality: number | null
  currentBandwidth?: number | null
  availableQualities: QualityOption[]
  autoMode: boolean
  onQualityChange: (height: number, bandwidth?: number) => void
  onAutoToggle: () => void
  showBitrateVariants?: boolean
}
