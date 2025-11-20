import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { MovieCard } from '@/components/media'
import { VideoPlayer } from '@/components/media/VideoPlayer'
import { MediaBrowsePage } from '@/components/common'
import { useMediaPlayback, useLibraryFilter, useInfiniteMovies, flattenMovies, BatchImagesProvider } from '@/lib/hooks'
import { logger } from '@/lib/utils/logger'

const Movies = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    id?: number
    q?: string
    sort?: string
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
      search: { id: undefined, q: search.q || undefined, sort: sort === 'title-asc' ? undefined : sort },
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
        onItemSelect={(movie) => handlePlayMovie(movie.id)}
        initialSearch={search.q || ''}
        initialSort={search.sort || 'title-asc'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
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

    return {
      id: parsedId && !isNaN(parsedId) ? parsedId : undefined,
      q,
      sort,
    }
  },
})
