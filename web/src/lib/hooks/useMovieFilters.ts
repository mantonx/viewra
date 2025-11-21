/**
 * useMovieFilters Hook
 * Manages movie-specific filters extracted from URL parameters
 */

import { useMemo } from 'react'

interface MovieFilters {
  searchQuery?: string
  sort?: string
  genres: string[]
  yearMin?: number
  yearMax?: number
  qualities: string[]
  watchedFilter: 'all' | 'watched' | 'unwatched'
}

interface MovieSearchParams {
  q?: string
  sort?: string
  genres?: string
  yearMin?: number
  yearMax?: number
  qualities?: string
  watched?: string
}

export const useMovieFilters = (searchParams: MovieSearchParams): MovieFilters => {
  return useMemo(() => ({
    searchQuery: searchParams.q,
    sort: searchParams.sort,
    genres: searchParams.genres ? searchParams.genres.split(',') : [],
    yearMin: searchParams.yearMin,
    yearMax: searchParams.yearMax,
    qualities: searchParams.qualities ? searchParams.qualities.split(',') : [],
    watchedFilter: (searchParams.watched as 'all' | 'watched' | 'unwatched') || 'all',
  }), [searchParams])
}
