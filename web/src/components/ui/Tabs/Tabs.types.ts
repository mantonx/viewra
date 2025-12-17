import type { ReactNode } from 'react'

export interface Tab {
  id: string
  label: ReactNode
  badge?: number | string
  disabled?: boolean
}

export interface TabsProps {
  tabs: Tab[]
  activeTab: string
  onTabChange: (tabId: string) => void
  variant?: 'default' | 'underline' | 'pills'
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export interface TabPanelProps {
  children: ReactNode
  isActive: boolean
  className?: string
}
