import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card, CardContent, Input, Select } from '@/components/ui'
import { MovieCard } from '@/components/media'
import { VideoPlayer } from '@/components/media/VideoPlayer'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { useMediaPlayback } from '@/lib/hooks/useMediaPlayback'
import { extractLibraries, getLibraryId, filterLibrariesByType } from '@/lib/utils/api'
import { moviesApi } from '@/lib/api/movies'
import { logger } from '@/lib/utils/logger'

const Movies = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as { id?: number }
  const urlMovieId = search.id
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Use the playback hook
  const { playbackState, playMedia, stopPlayback } = useMediaPlayback()

  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const libraries = extractLibraries(librariesData)

  // Filter to only movie libraries
  const movieLibraries = filterLibrariesByType(libraries, 'movies')

  // Get the library ID (defaults to first movie library when 'all' is selected)
  const libraryId = getLibraryId(selectedLibrary, libraries, 'movies')

  const {
    data: moviesData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['movies', libraryId],
    queryFn: () => moviesApi.listMovies(libraryId),
  })

  const allMovies = moviesData?.movies || []

  // Filter movies by search query
  const filteredMovies = allMovies.filter((movie) => {
    const matchesSearch =
      searchQuery === '' || movie.title.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesSearch
  })

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
    navigate({ to: '/movies', search: { id: movieId } })

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
      navigate({ to: '/movies', search: { id: undefined } })
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

  if (isLoading) {
    return <LoadingPage text="Loading movies..." />
  }

  if (error) {
    return <ErrorPage error={error} context="movies" />
  }

  return (
    <div className="p-8">
      <PageHeader title="Movies" description="Browse your movie collection." />

      {/* Filters */}
      <Card className="mb-6">
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search movies..."
            />
            <Select
              label="Library"
              value={selectedLibrary}
              onChange={(e) => setSelectedLibrary(e.target.value)}
              options={[
                { value: 'all', label: 'All Movie Libraries' },
                ...movieLibraries.map((lib) => ({
                  value: String(lib.id),
                  label: lib.name || '',
                })),
              ]}
            />
          </div>
        </CardContent>
      </Card>

      {/* Movies Grid */}
      {filteredMovies.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="🎬"
              title={allMovies.length === 0 ? 'No movies found' : 'No matches'}
              description={
                allMovies.length === 0
                  ? 'Add a library with movies and scan it to see your movies here.'
                  : 'No movies match your search. Try adjusting your query.'
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {filteredMovies.map((movie) => {
            return (
              <MovieCard
                key={movie.id}
                movie={movie}
                onClick={() => handlePlayMovie(movie.id)}
              />
            )
          })}
        </div>
      )}

      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {filteredMovies.length} of {allMovies.length} movies
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/movies/')({
  component: Movies,
  validateSearch: (search: Record<string, unknown>) => {
    const id = search.id
    const parsedId = typeof id === 'string' ? parseInt(id, 10) : typeof id === 'number' ? id : undefined
    return {
      id: parsedId && !isNaN(parsedId) ? parsedId : undefined,
    }
  },
})
