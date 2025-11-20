import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useMemo } from 'react'
import { MovieCard } from '@/components/movies'
import { MovieListItem } from '@/components/movies'
import { VideoPlayer } from '@/components/media'
import { MediaBrowsePage } from '@/components/common'
import { useMediaPlayback, useLibraryFilter, useInfiniteMovies, flattenMovies, BatchImagesProvider } from '@/lib/hooks'
import { logger } from '@/lib/utils/logger'
import type { FilterState, ViewMode } from '@/components/common'

const Movies = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    id?: number
    q?: string
    sort?: string
    genres?: string
    yearMin?: number
    yearMax?: number
    qualities?: string
    watched?: string
    view?: ViewMode
  }
  const urlMovieId = search.id

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('movies')

  // URL state handlers
  const handleSearchChange = (q: string) => {
    navigate({
      to: '/movies',
      search: { id: undefined, q: q || undefined, sort: search.sort || undefined },
      replace: true,
    })
  }

  const handleSortChange = (sort: string) => {
    navigate({
      to: '/movies',
      search: {
        id: undefined,
        q: search.q || undefined,
        sort: sort === 'title-asc' ? undefined : sort,
        genres: search.genres,
        yearMin: search.yearMin,
        yearMax: search.yearMax,
        qualities: search.qualities,
        watched: search.watched,
        view: search.view,
      },
      replace: true,
    })
  }

  const handleFiltersChange = (filters: FilterState) => {
    navigate({
      to: '/movies',
      search: {
        id: undefined,
        q: search.q || undefined,
        sort: search.sort || undefined,
        genres: filters.genres && filters.genres.length > 0 ? filters.genres.join(',') : undefined,
        yearMin: filters.yearMin,
        yearMax: filters.yearMax,
        qualities: filters.qualities && filters.qualities.length > 0 ? filters.qualities.join(',') : undefined,
        watched: filters.watchedFilter !== 'all' ? filters.watchedFilter : undefined,
        view: search.view,
      },
      replace: true,
    })
  }

  const handleViewModeChange = (viewMode: ViewMode) => {
    navigate({
      to: '/movies',
      search: {
        id: undefined,
        q: search.q || undefined,
        sort: search.sort || undefined,
        genres: search.genres,
        yearMin: search.yearMin,
        yearMax: search.yearMax,
        qualities: search.qualities,
        watched: search.watched,
        view: viewMode === 'grid' ? undefined : viewMode,
      },
      replace: true,
    })
  }

  // Use the playback hook
  const { playbackState, playMedia, stopPlayback } = useMediaPlayback()

  // Convert sort format from URL (title-asc) to API format (title_asc)
  const apiSort = search.sort?.replace(/-/g, '_')

  // Use infinite scroll for movies
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteMovies({ libraryId, sort: apiSort })

  const allMovies = data ? flattenMovies(data.pages) : []

  // Extract unique genres, year range, and quality options from movies
  const { genres, yearRange, qualityOptions } = useMemo(() => {
    const genreSet = new Set<string>()
    const qualitySet = new Set<string>()
    let minYear = Infinity
    let maxYear = -Infinity

    allMovies.forEach((movie) => {
      // Collect genres
      if (movie.genre) {
        movie.genre.forEach((g) => genreSet.add(g))
      }

      // Track year range
      if (movie.year) {
        minYear = Math.min(minYear, movie.year)
        maxYear = Math.max(maxYear, movie.year)
      }

      // Collect video qualities based on resolution
      if (movie.height) {
        if (movie.height >= 2160) qualitySet.add('4K')
        else if (movie.height >= 1080) qualitySet.add('1080p')
        else if (movie.height >= 720) qualitySet.add('720p')
        else qualitySet.add('SD')
      }
    })

    return {
      genres: Array.from(genreSet).sort(),
      yearRange: minYear !== Infinity ? { min: minYear, max: maxYear } : undefined,
      qualityOptions: Array.from(qualitySet).sort((a, b) => {
        const order = { '4K': 0, '1080p': 1, '720p': 2, 'SD': 3 }
        return (order[a as keyof typeof order] || 99) - (order[b as keyof typeof order] || 99)
      }),
    }
  }, [allMovies])

  // Parse initial filters from URL
  const initialFilters: FilterState = useMemo(() => ({
    genres: search.genres ? search.genres.split(',') : [],
    yearMin: search.yearMin,
    yearMax: search.yearMax,
    qualities: search.qualities ? search.qualities.split(',') : [],
    watchedFilter: (search.watched as 'all' | 'watched' | 'unwatched') || 'all',
  }), [search.genres, search.yearMin, search.yearMax, search.qualities, search.watched])

  // Infinite scroll: Detect when user scrolls near bottom
  const observerTarget = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
          fetchNextPage()
        }
      },
      { threshold: 0.1 }
    )

    const currentTarget = observerTarget.current
    if (currentTarget) {
      observer.observe(currentTarget)
    }

    return () => {
      if (currentTarget) {
        observer.unobserve(currentTarget)
      }
    }
  }, [fetchNextPage, hasNextPage, isFetchingNextPage])

  // Find currently playing movie
  const playingMovie = allMovies.find((m) => m.id === playbackState.mediaId)

  // Handle playing a movie
  const handlePlayMovie = async (movieId: number) => {
    logger.debug('🔔 handlePlayMovie called with movieId:', movieId)
    const movie = allMovies.find((m) => m.id === movieId)
    if (!movie) {
      logger.warn('❌ Movie not found for ID:', movieId)
      return
    }
    logger.debug('✅ Found movie:', movie.title)

    // Update URL with movie ID
    navigate({ to: '/movies', search: { id: movieId, q: undefined, sort: undefined } })

    // Convert Movie to the format expected by playMedia
    const mediaItem = {
      id: movie.id,
      library_id: movie.library_id,
      title: movie.title,
      file_path: movie.file_path,
      duration: movie.duration,
      type: 'movie' as const,
    }

    // Trigger playback via hook
    await playMedia(movieId, mediaItem)
  }

  // Handle closing the player
  const handleClosePlayer = () => {
    stopPlayback()
    // Clear URL parameter if present
    if (urlMovieId) {
      navigate({ to: '/movies', search: { id: undefined, q: search.q || undefined, sort: search.sort || undefined } })
    }
  }

  // If video player is showing, render it
  if (playbackState.isPlaying && playbackState.streamUrl && playingMovie) {
    return (
      <VideoPlayer
        mediaId={playingMovie.id}
        streamUrl={playbackState.streamUrl}
        initialPosition={playbackState.initialPosition}
        duration={playingMovie.duration}
        onClose={handleClosePlayer}
      />
    )
  }

  // Extract movie IDs for batch image loading
  const movieIds = allMovies.map((m) => m.id)

  return (
    <BatchImagesProvider mediaIds={movieIds}>
      <MediaBrowsePage
        type="movies"
        title="Movies"
        description="Browse your movie collection."
        searchPlaceholder="Search movies..."
        emptyIcon="🎬"
        emptyTitle="No movies found"
        emptyDescription="Add a library with movies and scan it to see your movies here."
        data={allMovies}
        isLoading={isLoading}
        error={error}
        renderItem={(movie) => (
          <MovieCard
            key={movie.id}
            movie={movie}
            onClick={() => handlePlayMovie(movie.id)}
          />
        )}
        renderListItem={(movie) => (
          <MovieListItem
            key={movie.id}
            movie={movie}
            onClick={() => handlePlayMovie(movie.id)}
          />
        )}
        onItemSelect={(movie) => handlePlayMovie(movie.id)}
        initialSearch={search.q || ''}
        initialSort={search.sort || 'title-asc'}
        initialFilters={initialFilters}
        initialViewMode={search.view || 'grid'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
        onFiltersChange={handleFiltersChange}
        onViewModeChange={handleViewModeChange}
        enableAdvancedFilters={true}
        genres={genres}
        yearRange={yearRange}
        qualityOptions={qualityOptions}
        showWatchedFilter={true}
        getItemGenres={(movie) => movie.genre}
        getItemYear={(movie) => movie.year}
        getItemQuality={(movie) => {
          if (!movie.height) return undefined
          if (movie.height >= 2160) return '4K'
          if (movie.height >= 1080) return '1080p'
          if (movie.height >= 720) return '720p'
          return 'SD'
        }}
      />
      {/* Infinite scroll observer target */}
      <div ref={observerTarget} className="h-20 flex items-center justify-center">
        {isFetchingNextPage && (
          <div className="text-gray-400">Loading more movies...</div>
        )}
      </div>
    </BatchImagesProvider>
  )
}

export const Route = createFileRoute('/_layout/movies/')({
  component: Movies,
  validateSearch: (search: Record<string, unknown>) => {
    const id = search.id
    const parsedId = typeof id === 'string' ? parseInt(id, 10) : typeof id === 'number' ? id : undefined
    const q = typeof search.q === 'string' ? search.q : undefined
    const sort = typeof search.sort === 'string' ? search.sort : undefined
    const genres = typeof search.genres === 'string' ? search.genres : undefined
    const yearMin = typeof search.yearMin === 'number' ? search.yearMin : typeof search.yearMin === 'string' ? parseInt(search.yearMin, 10) : undefined
    const yearMax = typeof search.yearMax === 'number' ? search.yearMax : typeof search.yearMax === 'string' ? parseInt(search.yearMax, 10) : undefined
    const qualities = typeof search.qualities === 'string' ? search.qualities : undefined
    const watched = typeof search.watched === 'string' ? search.watched : undefined
    const view = typeof search.view === 'string' && (search.view === 'grid' || search.view === 'list') ? search.view as ViewMode : undefined

    return {
      id: parsedId && !isNaN(parsedId) ? parsedId : undefined,
      q,
      sort,
      genres,
      yearMin: yearMin && !isNaN(yearMin) ? yearMin : undefined,
      yearMax: yearMax && !isNaN(yearMax) ? yearMax : undefined,
      qualities,
      watched,
      view,
    }
  },
})
