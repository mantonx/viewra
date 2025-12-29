import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, Button } from '@/components/ui'
import { TrackList } from '@/components/music'
import { EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { MediaPoster } from '@/components/media/MediaPoster'
import { musicApi } from '@/lib/api/music'
import { useAudioPlayer } from '@/lib/contexts/AudioPlayerContext'
import type { MusicTrackResponse } from '@/lib/types/music'

const AlbumDetail = () => {
  const navigate = useNavigate()
  const { albumId } = Route.useParams()
  const albumIdNum = parseInt(albumId, 10)
  const { playQueue, currentTrack } = useAudioPlayer()

  const {
    data: tracksData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['music-tracks-by-album-id', albumIdNum],
    queryFn: () => musicApi.listTracksByAlbumID(albumIdNum),
  })

  const tracks = (tracksData?.data && 'tracks' in tracksData.data) ? tracksData.data.tracks || [] : []

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

  // Handle clicking on a track - queue all tracks starting from clicked one
  const handleTrackClick = (track: MusicTrackResponse) => {
    const trackIndex = sortedTracks.findIndex((t) => t.id === track.id)
    if (trackIndex !== -1) {
      playQueue(sortedTracks as MusicTrackResponse[], trackIndex)
    } else {
      // Fallback to single track if not found (shouldn't happen)
      playQueue([track], 0)
    }
  }

  // Handle play all
  const handlePlayAll = () => {
    if (sortedTracks.length > 0) {
      playQueue(sortedTracks as MusicTrackResponse[], 0)
    }
  }

  // Handle back - navigate to artist detail if we have artist info
  const handleBack = () => {
    if (albumMetadata?.artist_id) {
      navigate({ to: `/music/artists/${albumMetadata.artist_id}` })
    } else {
      navigate({ to: '/music', search: { q: undefined, sort: undefined, view: undefined } })
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
          {albumMetadata?.artist_id ? 'Back to Artist' : 'Back to Music'}
        </Button>
      </div>

      {/* Album header */}
      <div className="mb-6">
        <div className="flex items-start gap-6">
          {/* Album art */}
          <div className="w-48 h-48 rounded-lg shadow-lg overflow-hidden shrink-0 bg-neutral-800">
            {albumIdNum && (
              <MediaPoster
                mediaId={albumIdNum}
                mediaType="music-album"
                alt={albumName}
                className="w-full h-full object-cover"
                preset="large"
                aspectRatio="square"
                fallbackIcon="💿"
              />
            )}
          </div>

          {/* Album info */}
          <div className="flex-1">
            <h1 className="text-3xl font-bold text-neutral-900 dark:text-neutral-50 mb-2">{albumName}</h1>
            <p className="text-lg text-neutral-600 dark:text-neutral-400 mb-2">{artistName}</p>
            {albumMetadata && (
              <div className="flex items-center gap-4 text-sm text-neutral-500 dark:text-neutral-500">
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
              tracks={sortedTracks as MusicTrackResponse[]}
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
