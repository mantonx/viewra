import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Music as MusicIcon } from 'lucide-react'
import { ArtistCard, ArtistListItem } from '@/components/music'
import { MediaBrowsePage, VirtualMediaGrid } from '@/components/common'
import { useLibraryFilter, useInfiniteArtists, flattenArtists, BatchImagesProvider, useDebounce } from '@/lib/hooks'
import type { ViewMode } from '@/components/common'
import type { GithubComMantonxViewraInternalApplicationMusicArtistSummary } from '@/lib/api/generated/models'
import type { ArtistSummary } from '@/lib/types/music'

const Music = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    q?: string
    sort?: string
    view?: ViewMode
  }

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('music')

  // URL state handlers
  const handleSearchChange = (q: string) => {
    navigate({
      to: '/music',
      search: { q: q || undefined, sort: search.sort || undefined, view: search.view },
      replace: true,
    })
  }

  const handleSortChange = (sort: string) => {
    navigate({
      to: '/music',
      search: { q: search.q || undefined, sort: sort === 'title-asc' ? undefined : sort, view: search.view },
      replace: true,
    })
  }

  const handleViewModeChange = (viewMode: ViewMode) => {
    navigate({
      to: '/music',
      search: { q: search.q || undefined, sort: search.sort || undefined, view: viewMode === 'grid' ? undefined : viewMode },
      replace: true,
    })
  }

  // Convert sort format from URL (title-asc) to API format (title_asc)
  const apiSort = (search.sort || 'title-asc').replace(/-/g, '_')

  // Debounce search query to avoid too many API calls while typing
  const debouncedSearch = useDebounce(search.q || '', 300)

  // Use infinite scroll for artists with server-side search
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteArtists({ libraryId, sort: apiSort, search: debouncedSearch, pageSize: 100 })

  const allArtists = data ? flattenArtists(data.pages) : []

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

  // Handle clicking on an artist card
  const handleArtistClick = (artistId: number) => {
    navigate({ to: `/music/artists/${artistId}` })
  }

  // Extract artist IDs for batch image loading
  const artistIds = allArtists.map((a) => a.id)

  return (
    <BatchImagesProvider entityIds={artistIds} mediaType="music_artist">
      <MediaBrowsePage
        type="music"
        title="Music"
        description="Browse your music collection by artist."
        searchPlaceholder="Search artists..."
        emptyIcon={MusicIcon}
        emptyTitle="No artists found"
        emptyDescription="Add a library with music and scan it to see your artists here."
        data={allArtists}
        isLoading={isLoading}
        error={error}
        renderItem={(artist: GithubComMantonxViewraInternalApplicationMusicArtistSummary) => (
          <ArtistCard
            key={artist.id}
            artist={artist}
            onClick={() => artist.id && handleArtistClick(artist.id)}
          />
        )}
        renderListItem={(artist: GithubComMantonxViewraInternalApplicationMusicArtistSummary) => (
          <ArtistListItem
            key={artist.id}
            artist={artist}
            onClick={() => handleArtistClick(artist.id)}
          />
        )}
        onItemSelect={(artist) => handleArtistClick(artist.id)}
        getItemSearchText={(artist) => artist.name || ''}
        initialSearch={search.q || ''}
        initialSort={search.sort || 'title-asc'}
        initialViewMode={search.view || 'grid'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
        onViewModeChange={handleViewModeChange}
        serverSideSearch={true}
        customGridRenderer={
          <VirtualMediaGrid
            items={allArtists}
            columns={columns}
            estimatedRowHeight={320}
            gap={16}
            renderItem={(artist) => (
              <ArtistCard
                key={artist.id}
                artist={artist as ArtistSummary}
                onClick={() => artist.id && handleArtistClick(artist.id)}
              />
            )}
            skeletonAspectRatio="square"
            fetchNextPage={fetchNextPage}
            hasNextPage={hasNextPage || false}
            isFetchingNextPage={isFetchingNextPage}
          />
        }
      />
    </BatchImagesProvider>
  )
}

export const Route = createFileRoute('/_layout/music/')({
  component: Music,
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
