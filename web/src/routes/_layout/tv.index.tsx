import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { TVShowCard, TVShowListItem } from '@/components/tv'
import { MediaBrowsePage, VirtualMediaGrid } from '@/components/common'
import { useLibraryFilter, useInfiniteTVShows, flattenTVShows, BatchImagesProvider } from '@/lib/hooks'
import { tvApi } from '@/lib/api/tv'
import type { ViewMode } from '@/components/common'
import type { GithubComMantonxViewraInternalApplicationTvTVShowSummary } from '@/lib/api/generated/models'
import type { TVShow } from '@/types/tv'

const TVShows = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    q?: string
    sort?: string
    view?: ViewMode
  }

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('tv')

  // URL state handlers
  const handleSearchChange = (q: string) => {
    navigate({
      to: '/tv',
      search: { q: q || undefined, sort: search.sort || undefined, view: search.view },
      replace: true,
    })
  }

  const handleSortChange = (sort: string) => {
    navigate({
      to: '/tv',
      search: { q: search.q || undefined, sort: sort === 'title-asc' ? undefined : sort, view: search.view },
      replace: true,
    })
  }

  const handleViewModeChange = (viewMode: ViewMode) => {
    navigate({
      to: '/tv',
      search: { q: search.q || undefined, sort: search.sort || undefined, view: viewMode === 'grid' ? undefined : viewMode },
      replace: true,
    })
  }

  // Convert sort format from URL (title-asc) to API format (title_asc)
  // Always use a sort value (default to title_asc) to ensure consistent query keys
  const apiSort = (search.sort || 'title-asc').replace(/-/g, '_')

  // Use infinite scroll for TV shows
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteTVShows({ libraryId, sort: apiSort })

  const allShows = data ? flattenTVShows(data.pages as Array<{ shows?: GithubComMantonxViewraInternalApplicationTvTVShowSummary[] }>) : []

  // Calculate responsive columns for virtualization
  const [columns, setColumns] = useState(6)
  useEffect(() => {
    const updateColumns = () => {
      const width = window.innerWidth
      if (width >= 1280) {
        setColumns(6) // xl
      } else if (width >= 1024) {
        setColumns(5) // lg
      } else if (width >= 768) {
        setColumns(4) // md
      } else if (width >= 640) {
        setColumns(3) // sm
      } else {
        setColumns(2) // base
      }
    }

    updateColumns()
    window.addEventListener('resize', updateColumns)
    return () => window.removeEventListener('resize', updateColumns)
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
        emptyIcon="📺"
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
        initialSearch={search.q}
        initialSort={search.sort || 'title-asc'}
        initialViewMode={search.view || 'grid'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
        onViewModeChange={handleViewModeChange}
        customGridRenderer={
          <VirtualMediaGrid
            items={allShows}
            columns={columns}
            estimatedRowHeight={450}
            gap={16}
            renderItem={(show) => (
              <TVShowCard
                key={show.id}
                show={show as TVShow}
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
    const view = typeof search.view === 'string' && (search.view === 'grid' || search.view === 'list') ? search.view as ViewMode : undefined

    return {
      q,
      sort,
      view,
    }
  },
})
