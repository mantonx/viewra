import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { TVShowCard, TVShowListItem } from '@/components/tv'
import { MediaBrowsePage } from '@/components/common'
import { useLibraryFilter, useInfiniteTVShows, flattenTVShows, BatchImagesProvider } from '@/lib/hooks'
import { tvApi } from '@/lib/api/tv'
import type { ViewMode } from '@/components/common'
import type { GithubComViewraViewraInternalApplicationTvTVShowSummary } from '@/lib/api/generated/models'

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

  const allShows = data ? flattenTVShows(data.pages) : []

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
        renderItem={(show: GithubComViewraViewraInternalApplicationTvTVShowSummary) => (
          <TVShowCard
            key={show.id}
            show={show}
            onClick={() => show.id && handleShowClick(show.id)}
            onPlay={() => show.id && handlePlayShow(show.id)}
          />
        )}
        renderListItem={(show: GithubComViewraViewraInternalApplicationTvTVShowSummary) => (
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
      />
      {/* Infinite scroll observer target */}
      <div ref={observerTarget} className="h-20 flex items-center justify-center">
        {isFetchingNextPage && (
          <div className="text-gray-400">Loading more shows...</div>
        )}
      </div>
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
