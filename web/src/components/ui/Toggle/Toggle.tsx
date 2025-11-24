import { cn } from '@/lib/utils'
import type { ToggleProps } from './Toggle.types'

export const Toggle = ({
  enabled,
  onChange,
  label,
  description,
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
        'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
        enabled ? 'bg-primary-600' : 'bg-neutral-300 dark:bg-neutral-700',
        disabled && 'opacity-50 cursor-not-allowed',
        !disabled && 'cursor-pointer',
        className
      )}
    >
      <span
        className={cn(
          'inline-block h-4 w-4 transform rounded-full bg-white shadow-lg transition-transform',
          enabled ? 'translate-x-6' : 'translate-x-1'
        )}
      />
    </button>
  )
}
