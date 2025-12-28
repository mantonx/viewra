import { Server, Film, Folder, Shield, Settings2 } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type CategoryConfig = {
  icon: LucideIcon
  label: string
  description: string
}

/**
 * System settings category configuration.
 * Maps category keys to their display properties.
 */
export const SYSTEM_CATEGORY_CONFIG: Record<string, CategoryConfig> = {
  server: {
    icon: Server,
    label: 'Server',
    description: 'Core server configuration',
  },
  transcoding: {
    icon: Film,
    label: 'Transcoding',
    description: 'Video transcoding and encoding options',
  },
  scanning: {
    icon: Folder,
    label: 'Library Scanning',
    description: 'File scanning and indexing behavior',
  },
  security: {
    icon: Shield,
    label: 'Security',
    description: 'Security and access policies',
  },
}

/**
 * Get category config with fallback for unknown categories.
 */
export const getCategoryConfig = (category: string): CategoryConfig => {
  return (
    SYSTEM_CATEGORY_CONFIG[category] || {
      icon: Settings2,
      label: category.charAt(0).toUpperCase() + category.slice(1),
      description: '',
    }
  )
}
