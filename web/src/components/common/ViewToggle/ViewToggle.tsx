import { bg, text, border } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ViewToggleProps } from './ViewToggle.types'

/**
 * View toggle component for switching between grid and list layouts
 */
export const ViewToggle = ({ value, onChange }: ViewToggleProps) => {
  return (
    <div className="flex gap-2" role="group" aria-label="View options">
      <button
        type="button"
        onClick={() => onChange('grid')}
        className={cn(
          'px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 min-w-11 flex items-center gap-2 cursor-pointer',
          value === 'grid'
            ? 'bg-primary-500 text-white border-primary-500'
            : cn(bg.elevated, text.secondary, border.secondary, 'hover:border-primary-300 dark:hover:border-primary-700')
        )}
        aria-pressed={value === 'grid'}
        aria-label="Grid view"
      >
        <svg
          className="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"
          />
        </svg>
        <span className="hidden sm:inline">Grid</span>
      </button>
      <button
        type="button"
        onClick={() => onChange('list')}
        className={cn(
          'px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 min-w-11 flex items-center gap-2 cursor-pointer',
          value === 'list'
            ? 'bg-primary-500 text-white border-primary-500'
            : cn(bg.elevated, text.secondary, border.secondary, 'hover:border-primary-300 dark:hover:border-primary-700')
        )}
        aria-pressed={value === 'list'}
        aria-label="List view"
      >
        <svg
          className="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 6h16M4 12h16M4 18h16"
          />
        </svg>
        <span className="hidden sm:inline">List</span>
      </button>
    </div>
  )
}
