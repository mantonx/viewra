import {
  HardDrive,
  Cloud,
  Brain,
  Compass,
  Globe,
  type LucideIcon,
} from 'lucide-react'

const iconMap: Record<string, LucideIcon> = {
  'hard-drive': HardDrive,
  cloud: Cloud,
  brain: Brain,
  compass: Compass,
  globe: Globe,
}

export const getIcon = (name: string | undefined): LucideIcon => {
  if (!name) {return Globe}
  return iconMap[name] || Globe
}
