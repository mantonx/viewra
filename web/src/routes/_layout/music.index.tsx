import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { ArtistCard } from '@/components/music'
import { MediaBrowsePage } from '@/components/common'
import { useLibraryFilter, useInfiniteArtists, flattenArtists, BatchImagesProvider } from '@/lib/hooks'

const Music = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as {
    q?: string
    sort?: string
  }

  // Use library filter to get the active library ID
  const { libraryId } = useLibraryFilter('music')

  // URL state handlers
  const handleSearchChange = (q: string) => {
    navigate({
      to: '/music',
      search: { q: q || undefined, sort: search.sort || undefined },
      replace: true,
    })
  }

  const handleSortChange = (sort: string) => {
    navigate({
      to: '/music',
      search: { q: search.q || undefined, sort: sort === 'title-asc' ? undefined : sort },
      replace: true,
    })
  }

  // Convert sort format from URL (title-asc) to API format (title_asc)
  // Always use a sort value (default to title_asc) to ensure consistent query keys
  const apiSort = (search.sort || 'title-asc').replace(/-/g, '_')

  // Use infinite scroll for artists
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteArtists({ libraryId, sort: apiSort })

  const allArtists = data ? flattenArtists(data.pages) : []

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
        emptyIcon="🎤"
        emptyTitle="No artists found"
        emptyDescription="Add a library with music and scan it to see your artists here."
        data={allArtists}
        isLoading={isLoading}
        error={error}
        renderItem={(artist) => (
          <ArtistCard
            key={artist.id}
            artist={artist}
            onClick={() => handleArtistClick(artist.id)}
          />
        )}
        onItemSelect={(artist) => handleArtistClick(artist.id)}
        initialSearch={search.q}
        initialSort={search.sort || 'title-asc'}
        onSearchChange={handleSearchChange}
        onSortChange={handleSortChange}
      />
      {/* Infinite scroll observer target */}
      <div ref={observerTarget} className="h-20 flex items-center justify-center">
        {isFetchingNextPage && (
          <div className="text-gray-400">Loading more artists...</div>
        )}
      </div>
    </BatchImagesProvider>
  )
}

export const Route = createFileRoute('/_layout/music/')({
  component: Music,
  validateSearch: (search: Record<string, unknown>) => {
    const q = typeof search.q === 'string' ? search.q : undefined
    const sort = typeof search.sort === 'string' ? search.sort : undefined

    return {
      q,
      sort,
    }
  },
})
