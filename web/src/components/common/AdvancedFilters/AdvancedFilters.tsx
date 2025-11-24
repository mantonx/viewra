import { Button } from '@/components/ui'
import { useState } from 'react'
import { bg, text, border } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { AdvancedFiltersProps } from './AdvancedFilters.types'

/**
 * Advanced filtering component for media browsing
 * Provides genre, year range, quality, and watched status filters
 */
export const AdvancedFilters = ({
  genres = [],
  yearRange,
  qualityOptions = [],
  showWatchedFilter = true,
  value,
  onChange,
}: AdvancedFiltersProps) => {
  const [isExpanded, setIsExpanded] = useState(false)

  const handleGenreToggle = (genre: string) => {
    const currentGenres = value.genres || []
    const newGenres = currentGenres.includes(genre)
      ? currentGenres.filter((g) => g !== genre)
      : [...currentGenres, genre]

    onChange({ ...value, genres: newGenres })
  }

  const handleYearChange = (type: 'min' | 'max', newValue: number) => {
    onChange({
      ...value,
      yearMin: type === 'min' ? newValue : value.yearMin,
      yearMax: type === 'max' ? newValue : value.yearMax,
    })
  }

  const handleQualityToggle = (quality: string) => {
    const currentQualities = value.qualities || []
    const newQualities = currentQualities.includes(quality)
      ? currentQualities.filter((q) => q !== quality)
      : [...currentQualities, quality]

    onChange({ ...value, qualities: newQualities })
  }

  const handleWatchedFilterChange = (watched: 'all' | 'watched' | 'unwatched') => {
    onChange({ ...value, watchedFilter: watched })
  }

  const clearFilters = () => {
    onChange({
      genres: [],
      yearMin: undefined,
      yearMax: undefined,
      qualities: [],
      watchedFilter: 'all',
    })
  }

  const hasActiveFilters =
    (value.genres && value.genres.length > 0) ||
    value.yearMin !== undefined ||
    value.yearMax !== undefined ||
    (value.qualities && value.qualities.length > 0) ||
    value.watchedFilter !== 'all'

  return (
    <div className="space-y-4">
      {/* Toggle Button */}
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setIsExpanded(!isExpanded)}
          className={cn(
            'flex items-center gap-2 text-sm font-medium transition-colors cursor-pointer',
            text.secondary,
            'hover:text-neutral-900 dark:hover:text-neutral-50'
          )}
          aria-expanded={isExpanded}
          aria-controls="advanced-filters-content"
        >
          <span>{isExpanded ? '▼' : '▶'}</span>
          <span>Advanced Filters</span>
          {hasActiveFilters && (
            <span className="px-2 py-0.5 text-xs bg-primary-100 dark:bg-primary-950 text-primary-700 dark:text-primary-300 rounded-full font-semibold">
              Active
            </span>
          )}
        </button>
        {hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters} className="text-xs">
            Clear All
          </Button>
        )}
      </div>

      {/* Expanded Filters */}
      {isExpanded && (
        <div id="advanced-filters-content" className={cn('space-y-6 pt-4 border-t', border.secondary)}>
          {/* Genre Filter */}
          {genres.length > 0 && (
            <div>
              <label className={cn('block text-sm font-medium mb-2', text.secondary)}>Genres</label>
              <div className="flex flex-wrap gap-2">
                {genres.map((genre) => (
                  <button
                    key={genre}
                    type="button"
                    onClick={() => handleGenreToggle(genre)}
                    className={cn(
                      'px-3 py-1.5 text-sm rounded-full border transition-colors min-h-11 cursor-pointer',
                      value.genres?.includes(genre)
                        ? 'bg-primary-500 text-white border-primary-500'
                        : cn(
                            bg.elevated,
                            text.secondary,
                            border.secondary,
                            'hover:border-primary-300 dark:hover:border-primary-700'
                          )
                    )}
                    aria-pressed={value.genres?.includes(genre)}
                  >
                    {genre}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Year Range Filter */}
          {yearRange && (
            <div>
              <label className={cn('block text-sm font-medium mb-2', text.secondary)}>Year Range</label>
              <div className="flex items-center gap-4">
                <div className="flex-1">
                  <label htmlFor="year-min" className={cn('block text-xs mb-1', text.tertiary)}>
                    From
                  </label>
                  <input
                    id="year-min"
                    type="number"
                    min={yearRange.min}
                    max={yearRange.max}
                    value={value.yearMin || yearRange.min}
                    onChange={(e) => handleYearChange('min', parseInt(e.target.value, 10))}
                    className={cn(
                      'w-full px-3 py-2.5 min-h-11 border rounded-md',
                      'focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent',
                      bg.elevated,
                      text.primary,
                      border.secondary
                    )}
                  />
                </div>
                <span className={cn('mt-6', text.tertiary)}>—</span>
                <div className="flex-1">
                  <label htmlFor="year-max" className={cn('block text-xs mb-1', text.tertiary)}>
                    To
                  </label>
                  <input
                    id="year-max"
                    type="number"
                    min={yearRange.min}
                    max={yearRange.max}
                    value={value.yearMax || yearRange.max}
                    onChange={(e) => handleYearChange('max', parseInt(e.target.value, 10))}
                    className={cn(
                      'w-full px-3 py-2.5 min-h-11 border rounded-md',
                      'focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent',
                      bg.elevated,
                      text.primary,
                      border.secondary
                    )}
                  />
                </div>
              </div>
            </div>
          )}

          {/* Quality Filter */}
          {qualityOptions.length > 0 && (
            <div>
              <label className={cn('block text-sm font-medium mb-2', text.secondary)}>Quality</label>
              <div className="flex flex-wrap gap-2">
                {qualityOptions.map((quality) => (
                  <button
                    key={quality}
                    type="button"
                    onClick={() => handleQualityToggle(quality)}
                    className={cn(
                      'px-3 py-1.5 text-sm rounded-full border transition-colors min-h-11 cursor-pointer',
                      value.qualities?.includes(quality)
                        ? 'bg-primary-500 text-white border-primary-500'
                        : cn(
                            bg.elevated,
                            text.secondary,
                            border.secondary,
                            'hover:border-primary-300 dark:hover:border-primary-700'
                          )
                    )}
                    aria-pressed={value.qualities?.includes(quality)}
                  >
                    {quality}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Watched/Unwatched Filter */}
          {showWatchedFilter && (
            <div>
              <label className={cn('block text-sm font-medium mb-2', text.secondary)}>Watch Status</label>
              <div className="flex gap-2">
                {(['all', 'watched', 'unwatched'] as const).map((status) => (
                  <button
                    key={status}
                    type="button"
                    onClick={() => handleWatchedFilterChange(status)}
                    className={cn(
                      'flex-1 px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 cursor-pointer',
                      value.watchedFilter === status
                        ? 'bg-primary-500 text-white border-primary-500'
                        : cn(
                            bg.elevated,
                            text.secondary,
                            border.secondary,
                            'hover:border-primary-300 dark:hover:border-primary-700'
                          )
                    )}
                    aria-pressed={value.watchedFilter === status}
                  >
                    {status.charAt(0).toUpperCase() + status.slice(1)}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
