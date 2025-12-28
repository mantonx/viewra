import { cn } from '@/lib/utils'
import { Check, Database, Eye, Lock } from 'lucide-react'

type SourceBadgeProps = {
  /**
   * Source of the setting value.
   * - 'default': Using default value
   * - 'database': Saved in database
   * - 'env_var': Set via environment variable
   * - 'detected': Auto-detected by system
   */
  source?: 'default' | 'database' | 'env_var' | 'detected' | string
  /** Environment variable name (shown when source is 'env_var') */
  envVar?: string
  /** Whether the setting is locked (typically when source is 'env_var') */
  locked?: boolean
  /** Whether the setting is read-only (typically when source is 'detected') */
  readOnly?: boolean
  /** Override to show as default (used in preferences for unsaved values) */
  isDefault?: boolean
}

/**
 * Badge indicating the source/state of a setting value.
 * Handles multiple states: default, saved, database, env_var (locked), detected.
 */
export const SourceBadge = ({ source, envVar, locked, readOnly, isDefault }: SourceBadgeProps) => {
  // Read-only/detected values
  if (readOnly || source === 'detected') {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400'
        )}
      >
        <Eye className="w-2.5 h-2.5" />
        detected
      </span>
    )
  }

  // Environment variable locked values
  if (source === 'env_var' && locked) {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono',
          'bg-amber-100 dark:bg-amber-900/50 text-amber-700 dark:text-amber-400'
        )}
      >
        <Lock className="w-2.5 h-2.5" />
        {envVar}
      </span>
    )
  }

  // Database saved values
  if (source === 'database') {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
        )}
      >
        <Database className="w-2.5 h-2.5" />
        saved
      </span>
    )
  }

  // Explicitly marked as default or no source specified
  if (isDefault || !source || source === 'default') {
    return (
      <span
        className={cn(
          'inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-neutral-100 dark:bg-neutral-800 text-neutral-400 dark:text-neutral-500'
        )}
      >
        Default
      </span>
    )
  }

  // Saved state (for preferences - when value differs from default)
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded',
        'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
      )}
    >
      <Check className="w-2.5 h-2.5" />
      Saved
    </span>
  )
}

export type { SourceBadgeProps }
