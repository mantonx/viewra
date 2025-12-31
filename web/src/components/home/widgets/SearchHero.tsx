import { useState, useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Search, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { glass } from '@/styles/semantic'
import type { Suggestion, SearchHeroData } from './widget.types'
import { SuggestionChip } from './SuggestionChip'

interface SearchHeroProps {
  data: SearchHeroData
  onSearch?: (query: string) => void
  className?: string
}

/**
 * SearchHero - Prominent search widget with contextual suggestion chips
 *
 * Displays a search input with suggestion chips based on user context
 * (time, weather, preferences, etc.). Works with semantic search when available.
 */
export const SearchHero = ({ data, onSearch, className }: SearchHeroProps) => {
  const [query, setQuery] = useState('')
  const [isFocused, setIsFocused] = useState(false)
  const navigate = useNavigate()

  const navigateToSearch = useCallback(
    (searchQuery: string) => {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
          q: searchQuery,
          sort: undefined,
          genres: undefined,
          yearMin: undefined,
          yearMax: undefined,
          qualities: undefined,
          watched: undefined,
          view: undefined,
        },
      })
    },
    [navigate]
  )

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      if (query.trim()) {
        if (onSearch) {
          onSearch(query.trim())
        } else {
          navigateToSearch(query.trim())
        }
      }
    },
    [query, onSearch, navigateToSearch]
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
            const filterParams = {
              id: undefined,
              t: undefined,
              q: undefined,
              sort: undefined,
              genres: suggestion.action.filter.genre || undefined,
              yearMin: undefined,
              yearMax: undefined,
              qualities: undefined,
              watched: undefined,
              view: undefined,
            }
            navigate({ to: '/movies', search: filterParams })
          }
          break
      }
    },
    [onSearch, navigate, navigateToSearch]
  )

  const handleClear = useCallback(() => {
    setQuery('')
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
                : 'border-neutral-200/50 dark:border-white/10'
            )}
          >
            <div className="flex items-center px-5 py-4">
              <Search className="w-5 h-5 text-neutral-400 dark:text-neutral-500 shrink-0" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onFocus={() => setIsFocused(true)}
                onBlur={() => setIsFocused(false)}
                placeholder={data.placeholder || 'Search your library...'}
                className={cn(
                  'flex-1 ml-3 bg-transparent outline-none text-lg',
                  'text-neutral-900 dark:text-white',
                  'placeholder:text-neutral-400 dark:placeholder:text-neutral-500'
                )}
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
        </form>

        {/* Suggestion chips */}
        {data.suggestions && data.suggestions.length > 0 && (
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
