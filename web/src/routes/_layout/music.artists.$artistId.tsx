import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, Button } from '@/components/ui'
import { AlbumCard } from '@/components/music'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { musicApi } from '@/lib/api/music'
import type { GithubComMantonxViewraInternalApplicationMusicAlbumSummary } from '@/lib/api/generated/models'

const ArtistDetail = () => {
  const navigate = useNavigate()
  const { artistId } = Route.useParams()
  const artistIdNum = parseInt(artistId, 10)

  const {
    data: albumsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['music-albums-by-artist-id', artistIdNum],
    queryFn: () => musicApi.listAlbumsByArtistID(artistIdNum),
  })

  const albums: GithubComMantonxViewraInternalApplicationMusicAlbumSummary[] =
    (albumsData?.data && 'albums' in albumsData.data) ? albumsData.data.albums || [] : []

  // Get artist name from first album
  const artistName = albums.length > 0 ? (albums[0].artist || 'Artist') : 'Artist'

  // Handle clicking on an album card
  const handleAlbumClick = (albumId: number) => {
    navigate({ to: `/music/albums/${albumId}` })
  }

  // Handle back to artists
  const handleBackToArtists = () => {
    navigate({ to: '/music', search: { q: undefined, sort: undefined, view: undefined } })
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
        title={artistName}
        description={`${albums.length} ${albums.length === 1 ? 'album' : 'albums'}`}
      />

      {/* Albums Grid */}
      {albums.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="💿"
              title="No albums found"
              description="This artist has no albums."
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {albums
            .filter((album) => album.id !== undefined)
            .map((album: GithubComMantonxViewraInternalApplicationMusicAlbumSummary) => (
              <AlbumCard
                key={album.id}
                album={album}
                onClick={() => handleAlbumClick(album.id as number)}
              />
            ))}
        </div>
      )}

      <div className="mt-4 text-sm text-neutral-500 dark:text-neutral-500 text-center">
        Showing {albums.length} {albums.length === 1 ? 'album' : 'albums'}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/music/artists/$artistId')({
  component: ArtistDetail,
})
