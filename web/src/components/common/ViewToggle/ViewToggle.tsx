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
        className={`px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 min-w-11 flex items-center gap-2 ${
          value === 'grid'
            ? 'bg-blue-600 text-white border-blue-600'
            : 'bg-white text-gray-700 border-gray-300 hover:border-blue-300 hover:bg-blue-50'
        }`}
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
        className={`px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 min-w-11 flex items-center gap-2 ${
          value === 'list'
            ? 'bg-blue-600 text-white border-blue-600'
            : 'bg-white text-gray-700 border-gray-300 hover:border-blue-300 hover:bg-blue-50'
        }`}
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
