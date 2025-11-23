import { Button } from '@/components/ui'
import { useState } from 'react'
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
          className="flex items-center gap-2 text-sm font-medium text-neutral-700 dark:text-neutral-300 hover:text-neutral-900 dark:hover:text-neutral-50 transition-colors"
          aria-expanded={isExpanded}
          aria-controls="advanced-filters-content"
        >
          <span>{isExpanded ? '▼' : '▶'}</span>
          <span>Advanced Filters</span>
          {hasActiveFilters && (
            <span className="px-2 py-0.5 text-xs bg-rose-100 dark:bg-rose-950 text-rose-700 dark:text-rose-300 rounded-full font-semibold">
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
        <div id="advanced-filters-content" className="space-y-6 pt-4 border-t border-neutral-200 dark:border-neutral-800">
          {/* Genre Filter */}
          {genres.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">Genres</label>
              <div className="flex flex-wrap gap-2">
                {genres.map((genre) => (
                  <button
                    key={genre}
                    type="button"
                    onClick={() => handleGenreToggle(genre)}
                    className={`px-3 py-1.5 text-sm rounded-full border transition-colors min-h-11 ${
                      value.genres?.includes(genre)
                        ? 'bg-rose-500 text-white border-rose-500'
                        : 'bg-white dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300 border-neutral-300 dark:border-neutral-700 hover:border-rose-300 hover:bg-rose-50 dark:hover:bg-rose-950'
                    }`}
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
              <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">Year Range</label>
              <div className="flex items-center gap-4">
                <div className="flex-1">
                  <label htmlFor="year-min" className="block text-xs text-neutral-600 dark:text-neutral-400 mb-1">
                    From
                  </label>
                  <input
                    id="year-min"
                    type="number"
                    min={yearRange.min}
                    max={yearRange.max}
                    value={value.yearMin || yearRange.min}
                    onChange={(e) => handleYearChange('min', parseInt(e.target.value, 10))}
                    className="w-full px-3 py-2.5 min-h-11 border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50 rounded-md focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-transparent"
                  />
                </div>
                <span className="text-neutral-400 dark:text-neutral-600 mt-6">—</span>
                <div className="flex-1">
                  <label htmlFor="year-max" className="block text-xs text-neutral-600 dark:text-neutral-400 mb-1">
                    To
                  </label>
                  <input
                    id="year-max"
                    type="number"
                    min={yearRange.min}
                    max={yearRange.max}
                    value={value.yearMax || yearRange.max}
                    onChange={(e) => handleYearChange('max', parseInt(e.target.value, 10))}
                    className="w-full px-3 py-2.5 min-h-[44px] border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50 rounded-md focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-transparent"
                  />
                </div>
              </div>
            </div>
          )}

          {/* Quality Filter */}
          {qualityOptions.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">Quality</label>
              <div className="flex flex-wrap gap-2">
                {qualityOptions.map((quality) => (
                  <button
                    key={quality}
                    type="button"
                    onClick={() => handleQualityToggle(quality)}
                    className={`px-3 py-1.5 text-sm rounded-full border transition-colors min-h-11 ${
                      value.qualities?.includes(quality)
                        ? 'bg-rose-500 text-white border-rose-500'
                        : 'bg-white dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300 border-neutral-300 dark:border-neutral-700 hover:border-rose-300 hover:bg-rose-50 dark:hover:bg-rose-950'
                    }`}
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
              <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">Watch Status</label>
              <div className="flex gap-2">
                {(['all', 'watched', 'unwatched'] as const).map((status) => (
                  <button
                    key={status}
                    type="button"
                    onClick={() => handleWatchedFilterChange(status)}
                    className={`flex-1 px-4 py-2.5 text-sm rounded-md border transition-colors min-h-11 ${
                      value.watchedFilter === status
                        ? 'bg-rose-500 text-white border-rose-500'
                        : 'bg-white dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300 border-neutral-300 dark:border-neutral-700 hover:border-rose-300 hover:bg-rose-50 dark:hover:bg-rose-950'
                    }`}
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
