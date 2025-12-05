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

export interface StatsPanelProps {
  stats: StreamStats | null
  networkStats: NetworkStats | null
  isVisible: boolean
  onClose: () => void
  isLoading?: boolean
}
