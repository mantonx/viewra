import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card, CardContent, Select, Button } from '@/components/ui'
import { AlbumCard } from '@/components/music'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { extractLibraries, getLibraryId, filterLibrariesByType } from '@/lib/utils/api'
import { musicApi } from '@/lib/api/music'

const ArtistDetail = () => {
  const navigate = useNavigate()
  const { artist } = Route.useParams()
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')

  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const libraries = extractLibraries(librariesData)
  
  // Filter to only music libraries
  const musicLibraries = filterLibrariesByType(libraries, 'music')

  // Get the library ID (defaults to first music library when 'all' is selected)
  const libraryId = getLibraryId(selectedLibrary, libraries, 'music')

  const {
    data: albumsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['music-albums', libraryId, artist],
    queryFn: () => musicApi.listAlbumsByArtist(libraryId, artist),
  })

  const albums = albumsData?.albums || []

  // Handle clicking on an album card
  const handleAlbumClick = (albumName: string) => {
    navigate({ to: `/music/${artist}/${albumName}` })
  }

  // Handle back to artists
  const handleBackToArtists = () => {
    navigate({ to: '/music' })
  }

  if (isLoading) {
    return <LoadingPage text="Loading albums..." />
  }

  if (error) {
    return <ErrorPage error={error} context="albums" />
  }

  return (
    <div className="p-8">
      {/* Back button */}
      <div className="mb-4">
        <Button onClick={handleBackToArtists} variant="secondary">
          <span className="mr-2">←</span>
          Back to Artists
        </Button>
      </div>

      <PageHeader
        title={decodeURIComponent(artist)}
        description={`${albums.length} ${albums.length === 1 ? 'album' : 'albums'}`}
      />

      {/* Library filter */}
      <Card className="mb-6">
        <CardContent>
          <Select
            label="Library"
            value={selectedLibrary}
            onChange={(e) => setSelectedLibrary(e.target.value)}
            options={[
              { value: 'all', label: 'All Music Libraries' },
              ...musicLibraries.map((lib) => ({
                value: String(lib.id),
                label: lib.name || '',
              })),
            ]}
          />
        </CardContent>
      </Card>

      {/* Albums Grid */}
      {albums.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="💿"
              title="No albums found"
              description="This artist has no albums in the selected library."
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {albums.map((album) => (
            <AlbumCard
              key={album.album}
              album={album}
              onClick={() => handleAlbumClick(album.album)}
            />
          ))}
        </div>
      )}

      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {albums.length} {albums.length === 1 ? 'album' : 'albums'}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/music/$artist')({
  component: ArtistDetail,
})
