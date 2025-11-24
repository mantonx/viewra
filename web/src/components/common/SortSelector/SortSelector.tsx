import { useState, useRef, useEffect } from 'react'
import { bg, text, border, shadow } from '@/styles/semantic'
import { cn } from '@/lib/utils'

export interface SortOption {
  field: string
  label: string
  enabled: boolean
}

export interface SortSelectorProps {
  value: string
  onChange: (value: string) => void
  className?: string
  showLabel?: boolean
}

const SORT_OPTIONS: SortOption[] = [
  { field: 'title', label: 'Title', enabled: true },
  { field: 'year', label: 'Year', enabled: true },
  { field: 'added', label: 'Date Added', enabled: true },
  { field: 'rating', label: 'Rating', enabled: true },
]

export const SortSelector = ({ value, onChange, className = '', showLabel = true }: SortSelectorProps) => {
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
      {showLabel && (
        <label className={cn('block text-sm font-medium mb-1', text.secondary)}>
          Sort By
        </label>
      )}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'w-full px-3 py-2.5 min-h-11 text-left rounded-md border flex items-center justify-between transition-colors',
          bg.elevated,
          text.primary,
          border.secondary,
          bg.hover.subtle,
          'focus:ring-2 focus:ring-blue-500 focus:border-blue-500'
        )}
        aria-label="Sort options"
        aria-expanded={isOpen}
        aria-haspopup="true"
      >
        <span className="text-sm">{getCurrentLabel()}</span>
        <svg
          className={cn('w-5 h-5 transition-transform', text.disabled, isOpen && 'rotate-180')}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className={cn('absolute z-50 mt-1 w-full rounded-md border', bg.elevated, border.secondary, shadow.lg)}>
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
                  className={cn(
                    'w-full px-4 py-2.5 min-h-11 text-left text-sm flex items-center justify-between transition-colors',
                    bg.hover.subtle,
                    isActive
                      ? 'bg-blue-50 dark:bg-blue-950 text-blue-700 dark:text-blue-300 font-medium'
                      : text.secondary
                  )}
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
                      className={cn(
                        'min-h-8 min-w-8 flex items-center justify-center rounded transition-colors',
                        'hover:bg-neutral-200 dark:hover:bg-neutral-700',
                        isActive ? 'text-blue-600 dark:text-blue-400' : text.disabled
                      )}
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
          <div className={cn('border-t px-4 py-2', border.primary, bg.secondary)}>
            <p className={cn('text-xs', text.tertiary)}>
              Click field to sort. Click arrow to toggle direction.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

export default SortSelector
