import { useState, useRef, useEffect } from 'react'

export interface SortOption {
  field: string
  label: string
  enabled: boolean
}

export interface SortSelectorProps {
  value: string
  onChange: (value: string) => void
  className?: string
}

const SORT_OPTIONS: SortOption[] = [
  { field: 'title', label: 'Title', enabled: true },
  { field: 'year', label: 'Year', enabled: true },
  { field: 'added', label: 'Date Added', enabled: true },
  { field: 'rating', label: 'Rating', enabled: true },
]

export const SortSelector = ({ value, onChange, className = '' }: SortSelectorProps) => {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const [field, direction] = value.split('-')

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen])

  const handleToggleSort = (sortField: string) => {
    // If clicking the same field, toggle direction
    if (sortField === field) {
      const newDirection = direction === 'asc' ? 'desc' : 'asc'
      onChange(`${sortField}-${newDirection}`)
    } else {
      // Default to descending for year, rating, added; ascending for title
      const defaultDirection = sortField === 'title' ? 'asc' : 'desc'
      onChange(`${sortField}-${defaultDirection}`)
    }
  }

  const getCurrentLabel = () => {
    const option = SORT_OPTIONS.find(opt => opt.field === field)
    const directionLabel = direction === 'asc' ? '↑' : '↓'
    return option ? `${option.label} ${directionLabel}` : 'Sort By'
  }

  return (
    <div ref={dropdownRef} className={`relative ${className}`}>
      <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1">
        Sort By
      </label>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-3 py-2.5 min-h-11 text-left bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500 flex items-center justify-between hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors text-neutral-900 dark:text-neutral-50"
        aria-label="Sort options"
        aria-expanded={isOpen}
        aria-haspopup="true"
      >
        <span className="text-sm">{getCurrentLabel()}</span>
        <svg
          className={`w-5 h-5 text-neutral-400 dark:text-neutral-600 transition-transform ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute z-50 mt-1 w-full bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 rounded-md shadow-lg dark:shadow-neutral-950/50">
          <div className="py-1">
            {SORT_OPTIONS.map((option) => {
              const isActive = option.field === field
              const currentDirection = isActive ? direction : (option.field === 'title' ? 'asc' : 'desc')

              return (
                <button
                  key={option.field}
                  type="button"
                  onClick={() => {
                    handleToggleSort(option.field)
                    setIsOpen(false)
                  }}
                  className={`w-full px-4 py-2.5 min-h-11 text-left text-sm hover:bg-neutral-50 dark:hover:bg-neutral-800 flex items-center justify-between transition-colors ${
                    isActive ? 'bg-blue-50 dark:bg-blue-950 text-blue-700 dark:text-blue-300 font-medium' : 'text-neutral-700 dark:text-neutral-300'
                  }`}
                >
                  <span>{option.label}</span>
                  <div className="flex items-center gap-2">
                    {isActive && (
                      <span className="px-1.5 py-0.5 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                        Active
                      </span>
                    )}
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        if (isActive) {
                          // Toggle direction without closing
                          const newDirection = direction === 'asc' ? 'desc' : 'asc'
                          onChange(`${field}-${newDirection}`)
                        } else {
                          handleToggleSort(option.field)
                          setIsOpen(false)
                        }
                      }}
                      className={`min-h-8 min-w-8 flex items-center justify-center rounded hover:bg-neutral-200 dark:hover:bg-neutral-700 transition-colors ${
                        isActive ? 'text-blue-600 dark:text-blue-400' : 'text-neutral-400 dark:text-neutral-600'
                      }`}
                      aria-label={`Sort by ${option.label} ${currentDirection === 'asc' ? 'ascending' : 'descending'}`}
                      title="Toggle sort direction"
                    >
                      {currentDirection === 'asc' ? (
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                        </svg>
                      ) : (
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      )}
                    </button>
                  </div>
                </button>
              )
            })}
          </div>
          <div className="border-t border-neutral-200 dark:border-neutral-800 px-4 py-2 bg-neutral-50 dark:bg-neutral-950">
            <p className="text-xs text-neutral-500 dark:text-neutral-500">
              Click field to sort. Click arrow to toggle direction.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

export default SortSelector
