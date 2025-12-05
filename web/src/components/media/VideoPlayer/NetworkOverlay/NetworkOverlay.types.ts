import type { NetworkStats } from '@/lib/network/NetworkMonitor'
import type { QualityDecision } from '@/lib/streaming/AutoQualityController'

export interface NetworkOverlayProps {
  stats: NetworkStats | null
  decision: QualityDecision | null
  currentQuality: string
  bufferLength: number
  sampleCount: number
  isVisible: boolean
  onClose: () => void
}
