import { cn } from '@/lib/utils'
import {
  Flame,
  CloudRain,
  Sun,
  Moon,
  Snowflake,
  Heart,
  Star,
  TrendingUp,
  Sparkles,
  PlusCircle,
  Award,
  Coffee,
  Zap,
  Compass,
  Laugh,
  Theater,
  Ghost,
  AlertTriangle,
  Rocket,
  Wand2,
  Search,
  Film,
  Palette,
  Users,
  Lightbulb,
  Wind,
  PartyPopper,
  Gift,
  Home,
  Flag,
  Calendar,
  Sunrise,
  Sofa,
  CloudLightning,
  Cloud,
  CloudFog,
  MoonStar,
  type LucideIcon,
} from 'lucide-react'
import type { Suggestion } from './widget.types'

interface SuggestionChipProps {
  suggestion: Suggestion
  onClick: () => void
  className?: string
}

/** Map icon names to Lucide components */
const iconMap: Record<string, LucideIcon> = {
  fire: Flame,
  flame: Flame,
  'cloud-rain': CloudRain,
  sun: Sun,
  moon: Moon,
  'moon-star': MoonStar,
  snowflake: Snowflake,
  heart: Heart,
  star: Star,
  'trending-up': TrendingUp,
  sparkles: Sparkles,
  'plus-circle': PlusCircle,
  award: Award,
  coffee: Coffee,
  zap: Zap,
  compass: Compass,
  laugh: Laugh,
  theater: Theater,
  ghost: Ghost,
  'alert-triangle': AlertTriangle,
  rocket: Rocket,
  wand: Wand2,
  search: Search,
  film: Film,
  palette: Palette,
  users: Users,
  lightbulb: Lightbulb,
  wind: Wind,
  'party-popper': PartyPopper,
  gift: Gift,
  home: Home,
  flag: Flag,
  calendar: Calendar,
  sunrise: Sunrise,
  sofa: Sofa,
  'cloud-lightning': CloudLightning,
  cloud: Cloud,
  'cloud-fog': CloudFog,
}

/**
 * SuggestionChip - Clickable suggestion chip for the search hero
 *
 * Renders a pill-shaped button with an optional icon and label.
 * Supports different styles: primary, secondary, accent
 */
export const SuggestionChip = ({ suggestion, onClick, className }: SuggestionChipProps) => {
  const Icon = suggestion.icon ? iconMap[suggestion.icon] : null

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'inline-flex items-center gap-2 px-4 py-2 rounded-full',
        'text-sm font-medium transition-all duration-200',
        'border backdrop-blur-sm',
        // Style variants
        suggestion.style === 'primary' && [
          'bg-primary-500/10 dark:bg-primary-500/20',
          'border-primary-500/30 dark:border-primary-500/40',
          'text-primary-700 dark:text-primary-300',
          'hover:bg-primary-500/20 dark:hover:bg-primary-500/30',
          'hover:border-primary-500/50',
          'hover:scale-105',
        ],
        suggestion.style === 'accent' && [
          'bg-purple-500/10 dark:bg-purple-500/20',
          'border-purple-500/30 dark:border-purple-500/40',
          'text-purple-700 dark:text-purple-300',
          'hover:bg-purple-500/20 dark:hover:bg-purple-500/30',
          'hover:border-purple-500/50',
          'hover:scale-105',
        ],
        // Default to secondary
        (!suggestion.style || suggestion.style === 'secondary') && [
          'bg-neutral-100/80 dark:bg-white/10',
          'border-neutral-200/50 dark:border-white/15',
          'text-neutral-700 dark:text-neutral-200',
          'hover:bg-neutral-200/80 dark:hover:bg-white/15',
          'hover:border-neutral-300/50 dark:hover:border-white/25',
          'hover:scale-105',
        ],
        className
      )}
    >
      {Icon && <Icon className="w-4 h-4" />}
      <span>{suggestion.label}</span>
    </button>
  )
}
