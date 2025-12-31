import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useMemo, useState, useCallback } from 'react'
import { Film } from 'lucide-react'
import { MovieCard } from '@/components/movies'
import { MovieListItem } from '@/components/movies'
import { VideoPlayerContainer } from '@/components/media'
import { MediaBrowsePage, VirtualMediaGrid } from '@/components/common'
import { useMediaPlayback, useLibraryFilter, useInfiniteMovies, flattenMovies, BatchImagesProvider, BatchProgressProvider, useDebounce } from '@/lib/hooks'
import { moviesApi } from '@/lib/api/movies'
import { logger } from '@/lib/utils/logger'
import type { FilterState, ViewMode } from '@/components/common'
import type { Movie } from '@/lib/types/movies'

const Movies = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    id?: number
    t?: number
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
  const urlTimePosition = search.t

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('movies')

  // URL state handlers
  const handleSearchChange = useCallback(
    (q: string) => {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
          q: q || undefined,
          sort: search.sort || undefined,
          genres: search.genres,
          yearMin: search.yearMin,
          yearMax: search.yearMax,
          qualities: search.qualities,
          watched: search.watched,
          view: search.view,
        },
        replace: true,
      })
    },
    [navigate, search.sort, search.genres, search.yearMin, search.yearMax, search.qualities, search.watched, search.view]
  )

  const handleSortChange = useCallback(
    (sort: string) => {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
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
    },
    [navigate, search.q, search.genres, search.yearMin, search.yearMax, search.qualities, search.watched, search.view]
  )

  const handleFiltersChange = useCallback(
    (filters: FilterState) => {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
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
    },
    [navigate, search.q, search.sort, search.view]
  )

  const handleViewModeChange = useCallback(
    (viewMode: ViewMode) => {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
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
    },
    [navigate, search.q, search.sort, search.genres, search.yearMin, search.yearMax, search.qualities, search.watched]
  )

  // Use the playback hook
  const { playbackState, playMedia, stopPlayback, changeQuality } = useMediaPlayback()

  // Track movie fetched by direct URL navigation (not in infinite scroll list)
  const [directPlayMovie, setDirectPlayMovie] = useState<Movie | null>(null)

  // Convert sort format from URL (title-asc) to API format (title_asc)
  const apiSort = search.sort?.replace(/-/g, '_')

  // Debounce search query to avoid too many API calls while typing
  const debouncedSearch = useDebounce(search.q || '', 300)

  // Use infinite scroll for movies with server-side search
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteMovies({ libraryId, sort: apiSort, search: debouncedSearch, pageSize: 100 })

  const allMovies = useMemo(() => data ? flattenMovies(data.pages) : [], [data])

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
        if (movie.height >= 2160) {qualitySet.add('4K')}
        else if (movie.height >= 1080) {qualitySet.add('1080p')}
        else if (movie.height >= 720) {qualitySet.add('720p')}
        else {qualitySet.add('SD')}
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

  // Calculate responsive columns and estimated row height for virtual grid
  const [columns, setColumns] = useState(6)
  const [estimatedRowHeight, setEstimatedRowHeight] = useState(600)

  useEffect(() => {
    const updateLayout = () => {
      const width = window.innerWidth
      let cols = 2
      let estimatedHeight = 400

      // Determine columns and rough height estimate based on viewport width
      if (width >= 1280) {
        cols = 6 // xl
        estimatedHeight = 600
      } else if (width >= 1024) {
        cols = 5 // lg
        estimatedHeight = 650
      } else if (width >= 768) {
        cols = 4 // md
        estimatedHeight = 700
      } else if (width >= 640) {
        cols = 3 // sm
        estimatedHeight = 800
      } else {
        cols = 2 // base
        estimatedHeight = 900
      }

      setColumns(cols)
      setEstimatedRowHeight(estimatedHeight)
    }

    updateLayout()
    window.addEventListener('resize', updateLayout)
    return () => window.removeEventListener('resize', updateLayout)
  }, [])

  // Find currently playing movie (check both loaded list and directly fetched movie)
  const playingMovie = allMovies.find((m) => m.id === playbackState.mediaId)
    || (directPlayMovie?.id === playbackState.mediaId ? directPlayMovie : null)

  // Handle playing a movie
  const handlePlayMovie = useCallback(async (movieId: number, startTime?: number) => {
    logger.debug('🔔 handlePlayMovie called with movieId:', movieId, 'startTime:', startTime)
    let movie = allMovies.find((m) => m.id === movieId)

    // If movie not in loaded list (e.g., direct URL navigation), fetch it
    if (!movie) {
      logger.debug('Movie not in list, fetching by ID:', movieId)
      try {
        const response = await moviesApi.getMovie(movieId)
        if (response.status !== 200 || !response.data) {
          logger.warn('❌ Movie not found for ID:', movieId)
          return
        }
        // Map the API response to our Movie type
        const movieData = response.data
        const fetchedMovie: Movie = {
          id: movieData.id ?? 0,
          library_id: movieData.library_id ?? 0,
          title: movieData.title ?? '',
          file_path: movieData.file_path ?? '',
          duration: movieData.duration ?? 0,
          year: movieData.year,
          genre: movieData.genre,
          height: movieData.height,
          width: movieData.width,
          is_extra: movieData.is_extra ?? false,
          created_at: movieData.created_at ?? '',
          updated_at: movieData.updated_at ?? '',
        }
        movie = fetchedMovie
        // Store the fetched movie so playingMovie can find it
        setDirectPlayMovie(fetchedMovie)
        logger.debug('✅ Fetched movie:', movie.title)
      } catch (error) {
        logger.error('❌ Failed to fetch movie:', error)
        return
      }
    } else {
      logger.debug('✅ Found movie in list:', movie.title)
      // Clear any previously fetched movie
      setDirectPlayMovie(null)
    }

    // Update URL with movie ID and optional time position
    // Preserve existing search params to avoid re-rendering the browse page
    navigate({
      to: '/movies',
      search: {
        id: movieId,
        t: startTime && startTime > 0 ? Math.floor(startTime) : undefined,
        q: search.q ?? undefined,
        sort: search.sort ?? undefined,
        genres: search.genres ?? undefined,
        yearMin: search.yearMin ?? undefined,
        yearMax: search.yearMax ?? undefined,
        qualities: search.qualities ?? undefined,
        watched: search.watched ?? undefined,
        view: search.view ?? undefined,
      }
    })

    // Convert Movie to the format expected by playMedia
    const mediaItem = {
      id: movie.id,
      library_id: movie.library_id,
      title: movie.title,
      file_path: movie.file_path,
      duration: movie.duration,
      type: 'movie' as const,
    }

    // Trigger playback via hook, passing URL time if available
    await playMedia(movieId, mediaItem, startTime)
  }, [allMovies, navigate, playMedia, setDirectPlayMovie, search.q, search.sort, search.genres, search.yearMin, search.yearMax, search.qualities, search.watched, search.view])

  // Handle time position updates from video player
  const handleTimeUpdate = useCallback((time: number) => {
    if (urlMovieId && time > 0) {
      navigate({
        to: '/movies',
        search: {
          id: urlMovieId,
          t: Math.floor(time),
          q: search.q || undefined,
          sort: search.sort || undefined,
          genres: search.genres,
          yearMin: search.yearMin,
          yearMax: search.yearMax,
          qualities: search.qualities,
          watched: search.watched,
          view: search.view,
        },
        replace: true, // Use replace to avoid polluting browser history
      })
    }
  }, [urlMovieId, navigate, search.q, search.sort, search.genres, search.yearMin, search.yearMax, search.qualities, search.watched, search.view])

  // Refs for auto-play control
  const lastPlayedIdRef = useRef<number | undefined>(undefined)
  const isClosingRef = useRef(false)

  // Handle closing the player
  const handleClosePlayer = () => {
    // Set closing flag to prevent auto-play effect from re-triggering
    isClosingRef.current = true
    stopPlayback()
    // Clear directly fetched movie
    setDirectPlayMovie(null)
    // Clear URL parameters if present
    if (urlMovieId) {
      navigate({
        to: '/movies',
        search: {
          id: undefined,
          t: undefined,
          q: search.q || undefined,
          sort: search.sort || undefined,
          genres: search.genres,
          yearMin: search.yearMin,
          yearMax: search.yearMax,
          qualities: search.qualities,
          watched: search.watched,
          view: search.view,
        },
      })
    }
  }

  // Auto-play when URL contains movie ID
  useEffect(() => {
    // Don't auto-play if we're in the process of closing
    if (isClosingRef.current) {
      return
    }
    if (urlMovieId && urlMovieId !== lastPlayedIdRef.current && !playbackState.isPlaying) {
      lastPlayedIdRef.current = urlMovieId
      handlePlayMovie(urlMovieId, urlTimePosition)
    }
  }, [urlMovieId, urlTimePosition, playbackState.isPlaying, handlePlayMovie])

  // Reset closing flag when URL movie ID is cleared
  useEffect(() => {
    if (!urlMovieId) {
      isClosingRef.current = false
    }
  }, [urlMovieId])

  // Render video player if playing
  const videoPlayer = (
    <VideoPlayerContainer
      playbackState={playbackState}
      media={playingMovie}
      onClose={handleClosePlayer}
      onTimeUpdate={handleTimeUpdate}
      onQualityChange={changeQuality}
    />
  )

  if (playbackState.isPlaying && playingMovie) {
    return videoPlayer
  }

  // Extract movie IDs for batch image loading
  const movieIds = allMovies.map((m) => m.id)

  return (
    <BatchImagesProvider mediaIds={movieIds}>
      <BatchProgressProvider mediaIds={movieIds}>
        <MediaBrowsePage
        type="movies"
        title="Movies"
        description="Browse your movie collection."
        searchPlaceholder="Search movies..."
        emptyIcon={Film}
        emptyTitle="No movies found"
        emptyDescription="Add a library with movies and scan it to see your movies here."
        data={allMovies}
        isLoading={isLoading}
        error={error}
        renderItem={(movie) => (
          <MovieCard
            key={movie.id}
            movie={movie as Movie}
            onClick={() => handlePlayMovie(movie.id)}
          />
        )}
        renderListItem={(movie) => (
          <MovieListItem
            key={movie.id}
            movie={movie as Movie}
            onClick={() => handlePlayMovie(movie.id)}
          />
        )}
        onItemSelect={(movie) => handlePlayMovie(movie.id)}
        getItemSearchText={(movie) => movie.title || ''}
        initialSearch={search.q || ''}
        initialSort={search.sort || 'title-asc'}
        initialFilters={initialFilters}
        initialViewMode={search.view || 'grid'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
        onFiltersChange={handleFiltersChange}
        onViewModeChange={handleViewModeChange}
        serverSideSearch={true}
        enableAdvancedFilters={true}
        genres={genres}
        yearRange={yearRange}
        qualityOptions={qualityOptions}
        showWatchedFilter={true}
        getItemGenres={(movie) => movie.genre}
        getItemYear={(movie) => movie.year}
        getItemQuality={(movie) => {
          if (!movie.height) {return undefined}
          if (movie.height >= 2160) {return '4K'}
          if (movie.height >= 1080) {return '1080p'}
          if (movie.height >= 720) {return '720p'}
          return 'SD'
        }}
        customGridRenderer={
          <VirtualMediaGrid
            items={allMovies}
            columns={columns}
            estimatedRowHeight={estimatedRowHeight}
            gap={16}
            renderItem={(movie) => (
              <MovieCard
                key={movie.id}
                movie={movie as Movie}
                onClick={() => handlePlayMovie(movie.id)}
              />
            )}
            skeletonAspectRatio="2/3"
            fetchNextPage={fetchNextPage}
            hasNextPage={hasNextPage || false}
            isFetchingNextPage={isFetchingNextPage}
          />
        }
      />
      </BatchProgressProvider>
    </BatchImagesProvider>
  )
}

export const Route = createFileRoute('/_layout/movies/')({
  component: Movies,
  validateSearch: (search: Record<string, unknown>) => {
    const id = search.id
    const parsedId = typeof id === 'string' ? parseInt(id, 10) : typeof id === 'number' ? id : undefined
    const t = search.t
    const parsedT = typeof t === 'string' ? parseInt(t, 10) : typeof t === 'number' ? t : undefined
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
      t: parsedT && !isNaN(parsedT) ? parsedT : undefined,
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
