import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, Button } from '@/components/ui'
import { TrackList } from '@/components/music'
import { EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { extractLibraries } from '@/lib/utils/api'
import { musicApi } from '@/lib/api/music'
import { useAudioPlayer } from '@/lib/contexts/AudioPlayerContext'
import type { MusicTrackResponse } from '@/lib/types/music'

const AlbumDetail = () => {
  const navigate = useNavigate()
  const { albumId } = Route.useParams()
  const albumIdNum = parseInt(albumId, 10)
  const { playTrack, playQueue, currentTrack } = useAudioPlayer()

  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const libraries = extractLibraries(librariesData)

  const {
    data: tracksData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['music-tracks-by-album-id', albumIdNum],
    queryFn: () => musicApi.listTracksByAlbumID(albumIdNum),
  })

  const tracks = tracksData?.tracks || []

  // Sort tracks by disc number and track number
  const sortedTracks = [...tracks].sort((a, b) => {
    const discA = a.disc_number || 1
    const discB = b.disc_number || 1
    if (discA !== discB) {
      return discA - discB
    }

    const trackA = a.track_number || 0
    const trackB = b.track_number || 0
    return trackA - trackB
  })

  // Get album metadata from first track
  const albumMetadata = tracks[0] || null
  const albumName = albumMetadata?.album || 'Album'
  const artistName = albumMetadata?.album_artist || albumMetadata?.artist || 'Unknown Artist'

  // Handle clicking on a track
  const handleTrackClick = (track: MusicTrackResponse) => {
    playTrack(track)
  }

  // Handle play all
  const handlePlayAll = () => {
    if (sortedTracks.length > 0) {
      playQueue(sortedTracks, 0)
    }
  }

  // Handle back - navigate to artist detail if we have artist info
  const handleBack = () => {
    if (albumMetadata) {
      // Try to navigate to artist using the first track's ID as artist ID
      // This is a bit of a hack but works since we need an artist track ID
      navigate({ to: '/music' })
    } else {
      navigate({ to: '/music' })
    }
  }

  if (isLoading) {
    return <LoadingPage text="Loading tracks..." />
  }

  if (error) {
    return <ErrorPage error={error} context="tracks" />
  }

  return (
    <div className="p-8">
      {/* Back button */}
      <div className="mb-4">
        <Button onClick={handleBack} variant="secondary">
          <span className="mr-2">←</span>
          Back to Music
        </Button>
      </div>

      {/* Album header */}
      <div className="mb-6">
        <div className="flex items-start gap-6">
          {/* Album art placeholder */}
          <div className="w-48 h-48 bg-linear-to-br from-purple-500 to-indigo-600 rounded-lg shadow-lg flex items-center justify-center text-white text-6xl shrink-0">
            <span role="img" aria-label="Album">
              💿
            </span>
          </div>

          {/* Album info */}
          <div className="flex-1">
            <h1 className="text-3xl font-bold text-gray-900 mb-2">{albumName}</h1>
            <p className="text-lg text-gray-600 mb-2">{artistName}</p>
            {albumMetadata && (
              <div className="flex items-center gap-4 text-sm text-gray-500">
                {albumMetadata.year && <span>{albumMetadata.year}</span>}
                <span>
                  {tracks.length} {tracks.length === 1 ? 'track' : 'tracks'}
                </span>
                {albumMetadata.genre && <span>{albumMetadata.genre}</span>}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Track List */}
      {tracks.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="🎵"
              title="No tracks found"
              description="This album has no tracks."
            />
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent>
            <TrackList
              tracks={sortedTracks}
              currentTrackId={currentTrack?.id}
              onTrackClick={handleTrackClick}
              onPlayAll={handlePlayAll}
            />
          </CardContent>
        </Card>
      )}
    </div>
  )
}

export const Route = createFileRoute('/_layout/music/albums/$albumId')({
  component: AlbumDetail,
})
