import { useState, useCallback, useRef, useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Search, X, Clock, Film, Tv, User } from 'lucide-react'
import { cn } from '@/lib/utils'
import { glass } from '@/styles/semantic'
import { useAutocomplete } from '@/lib/hooks'
import type { AutocompleteSuggestion } from '@/lib/hooks'
import type { Suggestion, SearchHeroData } from './widget.types'
import { SuggestionChip } from './SuggestionChip'

interface SearchHeroProps {
  data: SearchHeroData
  onSearch?: (query: string) => void
  className?: string
}

/**
 * Get icon for autocomplete suggestion based on type and subtype
 */
const getSuggestionIcon = (suggestion: AutocompleteSuggestion) => {
  if (suggestion.type === 'recent') {
    return <Clock className="w-4 h-4 text-neutral-400" />
  }
  if (suggestion.type === 'person') {
    return <User className="w-4 h-4 text-neutral-400" />
  }
  // Title type - check subtype
  if (suggestion.subtype === 'tv_show') {
    return <Tv className="w-4 h-4 text-neutral-400" />
  }
  return <Film className="w-4 h-4 text-neutral-400" />
}

/**
 * Get secondary text for autocomplete suggestion
 */
const getSuggestionSecondary = (suggestion: AutocompleteSuggestion) => {
  if (suggestion.type === 'recent') {
    return 'Recent search'
  }
  if (suggestion.type === 'person') {
    // Capitalize subtype: "director" -> "Director"
    return suggestion.subtype
      ? suggestion.subtype.charAt(0).toUpperCase() + suggestion.subtype.slice(1)
      : 'Person'
  }
  // Title type
  const parts: string[] = []
  if (suggestion.subtype === 'tv_show') {
    parts.push('TV Show')
  } else {
    parts.push('Movie')
  }
  if (suggestion.year) {
    parts.push(String(suggestion.year))
  }
  return parts.join(' · ')
}

/**
 * SearchHero - Prominent search widget with autocomplete dropdown
 *
 * Displays a search input with:
 * - Autocomplete dropdown with titles, people, and recent searches
 * - Keyboard navigation (up/down/enter/escape)
 * - Suggestion chips based on user context
 */
export const SearchHero = ({ data, onSearch, className }: SearchHeroProps) => {
  const [query, setQuery] = useState('')
  const [isFocused, setIsFocused] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Fetch autocomplete suggestions
  const { suggestions, isLoading, clear: clearSuggestions } = useAutocomplete(query, {
    debounceMs: 250,
    limit: 8,
  })

  // Show dropdown when focused, has query, and has suggestions (or loading)
  const showDropdown = isFocused && query.length >= 2 && (suggestions.length > 0 || isLoading)

  // Reset selected index when suggestions change
  useEffect(() => {
    setSelectedIndex(-1)
  }, [suggestions])

  const navigateToSearch = useCallback(
    (searchQuery: string) => {
      navigate({ to: `/movies?q=${encodeURIComponent(searchQuery)}` })
    },
    [navigate]
  )

  const navigateToMovie = useCallback(
    (movieId: number) => {
      navigate({ to: `/movies?id=${movieId}` })
    },
    [navigate]
  )

  const navigateToTVShow = useCallback(
    (showId: number) => {
      navigate({ to: `/tv/${showId}` })
    },
    [navigate]
  )

  /**
   * Handle selection of an autocomplete suggestion
   */
  const handleSelectSuggestion = useCallback(
    (suggestion: AutocompleteSuggestion) => {
      // Clear dropdown
      setQuery('')
      clearSuggestions()
      inputRef.current?.blur()

      if (suggestion.type === 'recent') {
        // Recent search - perform the search
        if (onSearch) {
          onSearch(suggestion.text)
        } else {
          navigateToSearch(suggestion.text)
        }
      } else if (suggestion.type === 'title') {
        // Title - navigate directly to the media
        if (suggestion.subtype === 'tv_show') {
          navigateToTVShow(suggestion.entity_id)
        } else {
          navigateToMovie(suggestion.entity_id)
        }
      } else if (suggestion.type === 'person') {
        // Person - search for their name
        if (onSearch) {
          onSearch(suggestion.text)
        } else {
          navigateToSearch(suggestion.text)
        }
      }
    },
    [onSearch, navigateToSearch, navigateToMovie, navigateToTVShow, clearSuggestions]
  )

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()

      // If an item is selected, use that
      if (selectedIndex >= 0 && selectedIndex < suggestions.length) {
        handleSelectSuggestion(suggestions[selectedIndex])
        return
      }

      // Otherwise submit the query
      if (query.trim()) {
        clearSuggestions()
        if (onSearch) {
          onSearch(query.trim())
        } else {
          navigateToSearch(query.trim())
        }
      }
    },
    [query, selectedIndex, suggestions, onSearch, navigateToSearch, handleSelectSuggestion, clearSuggestions]
  )

  /**
   * Handle keyboard navigation in dropdown
   */
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!showDropdown) {
        return
      }

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault()
          setSelectedIndex((prev) => (prev < suggestions.length - 1 ? prev + 1 : prev))
          break
        case 'ArrowUp':
          e.preventDefault()
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1))
          break
        case 'Escape':
          e.preventDefault()
          setSelectedIndex(-1)
          inputRef.current?.blur()
          break
        case 'Tab':
          // Allow tab to close dropdown naturally
          setSelectedIndex(-1)
          break
      }
    },
    [showDropdown, suggestions.length]
  )

  const handleSuggestionClick = useCallback(
    (suggestion: Suggestion) => {
      switch (suggestion.action.type) {
        case 'search':
          if (suggestion.action.query) {
            setQuery(suggestion.action.query)
            if (onSearch) {
              onSearch(suggestion.action.query)
            } else {
              navigateToSearch(suggestion.action.query)
            }
          }
          break
        case 'navigate':
          if (suggestion.action.url) {
            navigate({ to: suggestion.action.url as never })
          }
          break
        case 'filter':
          if (suggestion.action.filter) {
            const genre = suggestion.action.filter.genre
            if (genre) {
              navigate({ to: `/movies?genres=${encodeURIComponent(genre)}` })
            }
          }
          break
      }
    },
    [onSearch, navigate, navigateToSearch]
  )

  const handleClear = useCallback(() => {
    setQuery('')
    clearSuggestions()
    setSelectedIndex(-1)
    inputRef.current?.focus()
  }, [clearSuggestions])

  const handleFocus = useCallback(() => {
    setIsFocused(true)
  }, [])

  const handleBlur = useCallback((e: React.FocusEvent) => {
    // Don't close if clicking within the dropdown
    if (dropdownRef.current?.contains(e.relatedTarget as Node)) {
      return
    }
    setIsFocused(false)
    setSelectedIndex(-1)
  }, [])

  return (
    <div className={cn('relative', className)}>
      {/* Background glow effect */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[200px] bg-primary-500/10 dark:bg-primary-500/20 rounded-full blur-3xl" />
      </div>

      {/* Search container */}
      <div className="max-w-2xl mx-auto">
        {/* Search input */}
        <form onSubmit={handleSubmit} className="relative">
          <div
            className={cn(
              'relative rounded-2xl border transition-all duration-200',
              glass.medium.full,
              isFocused
                ? 'border-primary-500/50 shadow-lg shadow-primary-500/10'
                : 'border-neutral-200/50 dark:border-white/10',
              showDropdown && 'rounded-b-none border-b-0'
            )}
          >
            <div className="flex items-center px-5 py-4">
              <Search className="w-5 h-5 text-neutral-400 dark:text-neutral-500 shrink-0" />
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onFocus={handleFocus}
                onBlur={handleBlur}
                onKeyDown={handleKeyDown}
                placeholder={data.placeholder || 'Search your library...'}
                className={cn(
                  'flex-1 ml-3 bg-transparent outline-none text-lg',
                  'text-neutral-900 dark:text-white',
                  'placeholder:text-neutral-400 dark:placeholder:text-neutral-500'
                )}
                autoComplete="off"
                aria-autocomplete="list"
                aria-expanded={showDropdown}
                aria-controls={showDropdown ? 'autocomplete-listbox' : undefined}
                aria-activedescendant={
                  selectedIndex >= 0 ? `autocomplete-option-${selectedIndex}` : undefined
                }
              />
              {query && (
                <button
                  type="button"
                  onClick={handleClear}
                  className="p-1.5 rounded-full hover:bg-neutral-200/50 dark:hover:bg-white/10 transition-colors"
                >
                  <X className="w-4 h-4 text-neutral-400" />
                </button>
              )}
            </div>
          </div>

          {/* Autocomplete dropdown */}
          {showDropdown && (
            <div
              ref={dropdownRef}
              id="autocomplete-listbox"
              role="listbox"
              className={cn(
                'absolute left-0 right-0 z-50',
                'rounded-b-2xl border border-t-0 overflow-hidden',
                glass.medium.full,
                isFocused
                  ? 'border-primary-500/50 shadow-lg shadow-primary-500/10'
                  : 'border-neutral-200/50 dark:border-white/10'
              )}
            >
              {isLoading && suggestions.length === 0 ? (
                <div className="px-5 py-3 text-sm text-neutral-500">Searching...</div>
              ) : (
                <ul className="py-2">
                  {suggestions.map((suggestion, index) => (
                    <li
                      key={`${suggestion.type}-${suggestion.entity_id}-${index}`}
                      id={`autocomplete-option-${index}`}
                      role="option"
                      aria-selected={index === selectedIndex}
                      className={cn(
                        'flex items-center gap-3 px-5 py-2.5 cursor-pointer transition-colors',
                        index === selectedIndex
                          ? 'bg-primary-500/10 dark:bg-primary-500/20'
                          : 'hover:bg-neutral-100/50 dark:hover:bg-white/5'
                      )}
                      onMouseDown={(e) => {
                        // Prevent blur from firing before click
                        e.preventDefault()
                      }}
                      onClick={() => handleSelectSuggestion(suggestion)}
                      onMouseEnter={() => setSelectedIndex(index)}
                    >
                      {getSuggestionIcon(suggestion)}
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium text-neutral-900 dark:text-white truncate">
                          {suggestion.text}
                        </div>
                        <div className="text-xs text-neutral-500 dark:text-neutral-400">
                          {getSuggestionSecondary(suggestion)}
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </form>

        {/* Suggestion chips */}
        {data.suggestions && data.suggestions.length > 0 && !showDropdown && (
          <div className="mt-5">
            <div className="flex flex-wrap justify-center gap-2">
              {data.suggestions.map((suggestion) => (
                <SuggestionChip
                  key={suggestion.id}
                  suggestion={suggestion}
                  onClick={() => handleSuggestionClick(suggestion)}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
