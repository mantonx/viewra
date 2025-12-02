import { cn } from '@/lib/utils'
import type { ToggleProps } from './Toggle.types'

export const Toggle = ({
  enabled,
  onChange,
  label,
  description: _description,
  disabled = false,
  className = '',
}: ToggleProps) => {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={enabled}
      aria-label={label}
      disabled={disabled}
      onClick={() => !disabled && onChange(!enabled)}
      className={cn(
        'relative inline-flex h-6 w-11 items-center rounded-full transition-all duration-200',
        // Enabled state with glow
        enabled
          ? 'bg-primary-600 shadow-sm shadow-primary-500/30'
          : 'bg-neutral-300 dark:bg-neutral-700',
        // Disabled state
        disabled && 'opacity-50 cursor-not-allowed',
        !disabled && 'cursor-pointer',
        // Focus state
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-neutral-900',
        className
      )}
    >
      <span
        className={cn(
          'inline-block h-4 w-4 transform rounded-full bg-white transition-all duration-200',
          // Shadow and position
          enabled
            ? 'translate-x-6 shadow-md'
            : 'translate-x-1 shadow-sm'
        )}
      />
    </button>
  )
}
