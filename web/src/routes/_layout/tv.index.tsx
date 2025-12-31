import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState, useMemo } from 'react'
import { Tv } from 'lucide-react'
import { TVShowCard, TVShowListItem } from '@/components/tv'
import { MediaBrowsePage, VirtualMediaGrid } from '@/components/common'
import { useLibraryFilter, useInfiniteTVShows, flattenTVShows, BatchImagesProvider, useDebounce } from '@/lib/hooks'
import { tvApi } from '@/lib/api/tv'
import type { FilterState, ViewMode } from '@/components/common'
import type { GithubComMantonxViewraInternalApplicationTvTVShowSummary } from '@/lib/api/generated/models'
import type { TVShowSummary } from '@/lib/types/tv'

const TVShows = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    q?: string
    sort?: string
    genres?: string
    yearMin?: number
    yearMax?: number
    watched?: string
    view?: ViewMode
  }

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('tv')

  // URL state handlers
  const handleSearchChange = (q: string) => {
    navigate({
      to: '/tv',
      search: {
        q: q || undefined,
        sort: search.sort || undefined,
        genres: search.genres,
        yearMin: search.yearMin,
        yearMax: search.yearMax,
        watched: search.watched,
        view: search.view,
      },
      replace: true,
    })
  }

  const handleSortChange = (sort: string) => {
    navigate({
      to: '/tv',
      search: {
        q: search.q || undefined,
        sort: sort === 'title-asc' ? undefined : sort,
        genres: search.genres,
        yearMin: search.yearMin,
        yearMax: search.yearMax,
        watched: search.watched,
        view: search.view,
      },
      replace: true,
    })
  }

  const handleFiltersChange = (filters: FilterState) => {
    navigate({
      to: '/tv',
      search: {
        q: search.q || undefined,
        sort: search.sort || undefined,
        genres: filters.genres && filters.genres.length > 0 ? filters.genres.join(',') : undefined,
        yearMin: filters.yearMin,
        yearMax: filters.yearMax,
        watched: filters.watchedFilter !== 'all' ? filters.watchedFilter : undefined,
        view: search.view,
      },
      replace: true,
    })
  }

  const handleViewModeChange = (viewMode: ViewMode) => {
    navigate({
      to: '/tv',
      search: {
        q: search.q || undefined,
        sort: search.sort || undefined,
        genres: search.genres,
        yearMin: search.yearMin,
        yearMax: search.yearMax,
        watched: search.watched,
        view: viewMode === 'grid' ? undefined : viewMode,
      },
      replace: true,
    })
  }

  // Convert sort format from URL (title-asc) to API format (title_asc)
  // Always use a sort value (default to title_asc) to ensure consistent query keys
  const apiSort = (search.sort || 'title-asc').replace(/-/g, '_')

  // Debounce search query to avoid too many API calls while typing
  const debouncedSearch = useDebounce(search.q || '', 300)

  // Use infinite scroll for TV shows with server-side search
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteTVShows({ libraryId, sort: apiSort, search: debouncedSearch })

  const allShows = useMemo(() => {
    return data ? flattenTVShows(data.pages as Array<{ shows?: GithubComMantonxViewraInternalApplicationTvTVShowSummary[] }>) : []
  }, [data])

  // Extract unique genres and year range from shows
  const { genres, yearRange } = useMemo(() => {
    const genreSet = new Set<string>()
    let minYear = Infinity
    let maxYear = -Infinity

    allShows.forEach((show) => {
      // Collect genres
      if (show.genre) {
        show.genre.forEach((g) => genreSet.add(g))
      }

      // Track year range
      if (show.year) {
        minYear = Math.min(minYear, show.year)
        maxYear = Math.max(maxYear, show.year)
      }
    })

    return {
      genres: Array.from(genreSet).sort(),
      yearRange: minYear !== Infinity ? { min: minYear, max: maxYear } : undefined,
    }
  }, [allShows])

  // Parse initial filters from URL
  const initialFilters: FilterState = useMemo(() => ({
    genres: search.genres ? search.genres.split(',') : [],
    yearMin: search.yearMin,
    yearMax: search.yearMax,
    qualities: [],
    watchedFilter: (search.watched as 'all' | 'watched' | 'unwatched') || 'all',
  }), [search.genres, search.yearMin, search.yearMax, search.watched])

  // Calculate responsive columns and estimated row height for virtualization
  const [columns, setColumns] = useState(6)
  const [estimatedRowHeight, setEstimatedRowHeight] = useState(550)

  useEffect(() => {
    const updateLayout = () => {
      const width = window.innerWidth
      let cols = 2
      let estimatedHeight = 400

      // Determine columns and rough height estimate based on viewport width
      if (width >= 1280) {
        cols = 6 // xl
        estimatedHeight = 550
      } else if (width >= 1024) {
        cols = 5 // lg
        estimatedHeight = 600
      } else if (width >= 768) {
        cols = 4 // md
        estimatedHeight = 650
      } else if (width >= 640) {
        cols = 3 // sm
        estimatedHeight = 750
      } else {
        cols = 2 // base
        estimatedHeight = 850
      }

      setColumns(cols)
      setEstimatedRowHeight(estimatedHeight)
    }

    updateLayout()
    window.addEventListener('resize', updateLayout)
    return () => window.removeEventListener('resize', updateLayout)
  }, [])

  // Handle clicking on a show card
  const handleShowClick = (showId: number) => {
    navigate({ to: `/tv/${showId}` })
  }

  // Handle playing a show (play the next episode to watch)
  const handlePlayShow = async (showId: number) => {
    try {
      const nextEpisode = await tvApi.getNextEpisode(showId)
      // Navigate to the season page with the episode ID to auto-play
      navigate({
        to: `/tv/${showId}/season/${nextEpisode.season}`,
        search: { episodeId: nextEpisode.id }
      })
    } catch (error) {
      console.error('Failed to get next episode:', error)
      // Fallback: just navigate to the show page
      navigate({ to: `/tv/${showId}` })
    }
  }

  // Extract show IDs for batch image loading
  const showIds = allShows.map((s) => s.id)

  return (
    <BatchImagesProvider entityIds={showIds} mediaType="tv_show">
      <MediaBrowsePage
        type="tv"
        title="TV Shows"
        description="Browse your TV show collection."
        searchPlaceholder="Search TV shows..."
        emptyIcon={Tv}
        emptyTitle="No TV shows found"
        emptyDescription="Add a library with TV shows and scan it to see your shows here."
        data={allShows}
        isLoading={isLoading}
        error={error}
        renderItem={(show: GithubComMantonxViewraInternalApplicationTvTVShowSummary) => (
          <TVShowCard
            key={show.id}
            show={show}
            onClick={() => show.id && handleShowClick(show.id)}
            onPlay={() => show.id && handlePlayShow(show.id)}
          />
        )}
        renderListItem={(show: GithubComMantonxViewraInternalApplicationTvTVShowSummary) => (
          <TVShowListItem
            key={show.id}
            show={show}
            onClick={() => handleShowClick(show.id)}
          />
        )}
        onItemSelect={(show) => handleShowClick(show.id)}
        getItemSearchText={(show) => show.title || ''}
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
        showWatchedFilter={true}
        getItemGenres={(show) => show.genre}
        getItemYear={(show) => show.year}
        customGridRenderer={
          <VirtualMediaGrid
            items={allShows}
            columns={columns}
            estimatedRowHeight={estimatedRowHeight}
            gap={16}
            renderItem={(show) => (
              <TVShowCard
                key={show.id}
                show={show as TVShowSummary}
                onClick={() => show.id && handleShowClick(show.id)}
                onPlay={() => show.id && handlePlayShow(show.id)}
              />
            )}
            skeletonAspectRatio="2/3"
            fetchNextPage={fetchNextPage}
            hasNextPage={hasNextPage || false}
            isFetchingNextPage={isFetchingNextPage}
          />
        }
      />
    </BatchImagesProvider>
  )
}

export const Route = createFileRoute('/_layout/tv/')({
  component: TVShows,
  validateSearch: (search: Record<string, unknown>) => {
    const q = typeof search.q === 'string' ? search.q : undefined
    const sort = typeof search.sort === 'string' ? search.sort : undefined
    const genres = typeof search.genres === 'string' ? search.genres : undefined
    const yearMin = typeof search.yearMin === 'number' ? search.yearMin : typeof search.yearMin === 'string' ? parseInt(search.yearMin, 10) : undefined
    const yearMax = typeof search.yearMax === 'number' ? search.yearMax : typeof search.yearMax === 'string' ? parseInt(search.yearMax, 10) : undefined
    const watched = typeof search.watched === 'string' ? search.watched : undefined
    const view = typeof search.view === 'string' && (search.view === 'grid' || search.view === 'list') ? search.view as ViewMode : undefined

    return {
      q,
      sort,
      genres,
      yearMin: yearMin && !isNaN(yearMin) ? yearMin : undefined,
      yearMax: yearMax && !isNaN(yearMax) ? yearMax : undefined,
      watched,
      view,
    }
  },
})
