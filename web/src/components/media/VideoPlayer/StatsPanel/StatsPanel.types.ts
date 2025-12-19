import type { StreamStats } from '@/lib/types/streamStats'
import type { NetworkStats } from '@/lib/network/NetworkMonitor'

export interface StatRowProps {
  label: string
  value: string | number | undefined | null
  valueColor?: string
  subValue?: string
}

export interface SectionProps {
  title: string
  children: React.ReactNode
  defaultOpen?: boolean
}

/** Selected quality info for display in stats panel */
export interface SelectedQualityInfo {
  id: string              // Quality ID (e.g., "1080p-10m", "original")
  displayName: string     // Human-readable (e.g., "1080p (10 Mbps)")
  height: number
  bandwidth: number
  isOriginalQuality: boolean // True if this is the source quality
}

export interface StatsPanelProps {
  stats: StreamStats | null
  networkStats: NetworkStats | null
  isVisible: boolean
  onClose: () => void
  isLoading?: boolean
  /** Currently selected quality for display */
  selectedQuality?: SelectedQualityInfo | null
}
