import { cn } from '@/lib/utils'
import {
  X,
  Clock,
  User,
  Film,
  Building2,
  Globe,
  Sparkles,
  Ban,
  Layers,
  Palette,
  type LucideIcon,
} from 'lucide-react'

export interface IntentChipType {
  id: string
  type: string
  value: string
  display: string
  removable: boolean
  refinements?: string[]
  role?: string
}

interface IntentChipProps {
  chip: IntentChipType
  onRemove?: (chipId: string) => void
  onRefine?: (chipId: string, refinement: string) => void
  className?: string
}

/** Map chip types to icons */
const typeIconMap: Record<string, LucideIcon> = {
  decade: Clock,
  person: User,
  genre: Film,
  studio: Building2,
  language: Globe,
  mood: Sparkles,
  similar_to: Layers,
  collection: Layers,
  playback: Palette,
  exclusion: Ban,
}

/** Map chip types to color schemes */
const typeColorMap: Record<
  string,
  {
    bg: string
    border: string
    text: string
    hover: string
  }
> = {
  decade: {
    bg: 'bg-blue-500/10 dark:bg-blue-500/20',
    border: 'border-blue-500/30 dark:border-blue-500/40',
    text: 'text-blue-700 dark:text-blue-300',
    hover: 'hover:bg-blue-500/15 dark:hover:bg-blue-500/25',
  },
  person: {
    bg: 'bg-purple-500/10 dark:bg-purple-500/20',
    border: 'border-purple-500/30 dark:border-purple-500/40',
    text: 'text-purple-700 dark:text-purple-300',
    hover: 'hover:bg-purple-500/15 dark:hover:bg-purple-500/25',
  },
  genre: {
    bg: 'bg-green-500/10 dark:bg-green-500/20',
    border: 'border-green-500/30 dark:border-green-500/40',
    text: 'text-green-700 dark:text-green-300',
    hover: 'hover:bg-green-500/15 dark:hover:bg-green-500/25',
  },
  studio: {
    bg: 'bg-orange-500/10 dark:bg-orange-500/20',
    border: 'border-orange-500/30 dark:border-orange-500/40',
    text: 'text-orange-700 dark:text-orange-300',
    hover: 'hover:bg-orange-500/15 dark:hover:bg-orange-500/25',
  },
  language: {
    bg: 'bg-cyan-500/10 dark:bg-cyan-500/20',
    border: 'border-cyan-500/30 dark:border-cyan-500/40',
    text: 'text-cyan-700 dark:text-cyan-300',
    hover: 'hover:bg-cyan-500/15 dark:hover:bg-cyan-500/25',
  },
  mood: {
    bg: 'bg-pink-500/10 dark:bg-pink-500/20',
    border: 'border-pink-500/30 dark:border-pink-500/40',
    text: 'text-pink-700 dark:text-pink-300',
    hover: 'hover:bg-pink-500/15 dark:hover:bg-pink-500/25',
  },
  similar_to: {
    bg: 'bg-primary-500/10 dark:bg-primary-500/20',
    border: 'border-primary-500/30 dark:border-primary-500/40',
    text: 'text-primary-700 dark:text-primary-300',
    hover: 'hover:bg-primary-500/15 dark:hover:bg-primary-500/25',
  },
  collection: {
    bg: 'bg-indigo-500/10 dark:bg-indigo-500/20',
    border: 'border-indigo-500/30 dark:border-indigo-500/40',
    text: 'text-indigo-700 dark:text-indigo-300',
    hover: 'hover:bg-indigo-500/15 dark:hover:bg-indigo-500/25',
  },
  playback: {
    bg: 'bg-amber-500/10 dark:bg-amber-500/20',
    border: 'border-amber-500/30 dark:border-amber-500/40',
    text: 'text-amber-700 dark:text-amber-300',
    hover: 'hover:bg-amber-500/15 dark:hover:bg-amber-500/25',
  },
  exclusion: {
    bg: 'bg-red-500/10 dark:bg-red-500/20',
    border: 'border-red-500/30 dark:border-red-500/40',
    text: 'text-red-700 dark:text-red-300',
    hover: 'hover:bg-red-500/15 dark:hover:bg-red-500/25',
  },
}

/**
 * IntentChip - Displays a detected query intent with optional removal
 *
 * Shows users what the system understood from their search query.
 * Chips can be removed to refine the search.
 */
export const IntentChip = ({ chip, onRemove, onRefine, className }: IntentChipProps) => {
  const Icon = typeIconMap[chip.type] || Sparkles
  const colors = typeColorMap[chip.type] || typeColorMap.mood

  const handleRemove = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (onRemove) {
      onRemove(chip.id)
    }
  }

  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full',
        'text-sm font-medium transition-all duration-200',
        'border backdrop-blur-sm',
        colors.bg,
        colors.border,
        colors.text,
        chip.removable && 'pr-1.5',
        className
      )}
    >
      <Icon className="w-3.5 h-3.5 shrink-0" />
      <span className="whitespace-nowrap">
        {chip.display}
        {chip.role && (
          <span className="ml-1 text-xs opacity-70">({chip.role})</span>
        )}
      </span>
      {chip.removable && onRemove && (
        <button
          type="button"
          onClick={handleRemove}
          className={cn(
            'ml-1 p-0.5 rounded-full transition-colors',
            colors.hover,
            'hover:scale-110'
          )}
          aria-label={`Remove ${chip.display} filter`}
        >
          <X className="w-3 h-3" />
        </button>
      )}
    </div>
  )
}
