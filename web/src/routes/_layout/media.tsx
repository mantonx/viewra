import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiMediaQueryOptions, getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { Card, CardContent, Input, Select, Alert, Loading } from '@/components/ui'
import { MediaCard } from '@/components/media'
import { VideoPlayer } from '@/components/media/VideoPlayer'
import { PageHeader, EmptyState } from '@/components/common'
import { useMediaPlayback } from '@/lib/hooks/useMediaPlayback'

const Media = () => {
  const navigate = useNavigate()
  const search = Route.useSearch() as { id?: number }
  const urlMediaId = search.id
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Use the playback hook
  const { playbackState, playMedia, stopPlayback } = useMediaPlayback()

  const {
    data: mediaData,
    isLoading: isLoadingMedia,
    error: mediaError,
  } = useQuery(getGetApiMediaQueryOptions())
  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())

  const libraries = ((librariesData && 'libraries' in librariesData ? librariesData.libraries : []) || []) as any[]
  const allMedia = ((mediaData && 'media' in mediaData ? mediaData.media : []) || []) as any[]

  // Find currently playing media
  const playingMedia = playbackState.mediaId ? allMedia.find((m: any) => m.id === playbackState.mediaId) : null

  // Filter media by selected library and search query
  const filteredMedia = allMedia.filter((item: any) => {
    const matchesLibrary = selectedLibrary === 'all' || item.library_id === selectedLibrary
    const matchesSearch =
      searchQuery === '' || item.title.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesLibrary && matchesSearch
  })

  // Auto-play media if ID is in URL (only on initial load)
  useEffect(() => {
    if (urlMediaId && !playbackState.isPlaying && !playbackState.mediaId && allMedia.length > 0) {
      const media = allMedia.find((m: any) => m.id === urlMediaId)
      if (media) {
        handlePlayMedia(urlMediaId)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [urlMediaId, allMedia.length])

  // Handle playing a media item
  const handlePlayMedia = async (mediaId: number) => {
    console.log('🔔 handlePlayMedia called with mediaId:', mediaId)
    const media = allMedia.find((m: any) => m.id === mediaId)
    if (!media) {
      console.warn('❌ Media not found for ID:', mediaId)
      return
    }
    console.log('✅ Found media:', media.title)

    // Update URL with media ID
    navigate({ to: '/media', search: { id: mediaId } })

    // Trigger playback via hook
    await playMedia(mediaId, media)
  }

  // Handle closing the player
  const handleClosePlayer = () => {
    stopPlayback()
    // Clear URL parameter if present
    if (urlMediaId) {
      navigate({ to: '/media', search: {} })
    }
  }

  // If transcoding in progress, show loading state
  if (playbackState.isPlaying && !playbackState.streamUrl && playbackState.transcodeState === 'transcoding') {
    return (
      <div className="fixed inset-0 z-50 bg-black flex items-center justify-center">
        <div className="text-center">
          <Loading size="lg" text="Preparing video for playback..." />
          <p className="text-white mt-4 text-sm">This may take a few moments</p>
          <button
            onClick={handleClosePlayer}
            className="mt-6 px-4 py-2 bg-white/20 text-white rounded hover:bg-white/30"
          >
            Cancel
          </button>
        </div>
      </div>
    )
  }

  // If video player is showing, render it
  if (playbackState.isPlaying && playbackState.streamUrl && playingMedia) {
    return (
      <VideoPlayer
        mediaId={playingMedia.id || 0}
        streamUrl={playbackState.streamUrl}
        initialPosition={playbackState.initialPosition}
        duration={playingMedia.duration}
        onClose={handleClosePlayer}
      />
    )
  }

  if (isLoadingMedia) {
    return (
      <div className="p-8">
        <Loading size="lg" text="Loading media..." />
      </div>
    )
  }

  if (mediaError) {
    return (
      <div className="p-8">
        <Alert variant="error">Error loading media: {mediaError.message}</Alert>
      </div>
    )
  }

  return (
    <div className="p-8">
      <PageHeader title="Media" description="Browse your media collection." />

      {/* Filters */}
      <Card className="mb-6">
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by title..."
            />
            <Select
              label="Library"
              value={selectedLibrary}
              onChange={(e) => setSelectedLibrary(e.target.value)}
              options={[
                { value: 'all', label: 'All Libraries' },
                ...libraries.map((lib: any) => ({
                  value: String(lib.id),
                  label: lib.name || '',
                })),
              ]}
            />
          </div>
        </CardContent>
      </Card>

      {/* Media Grid */}
      {filteredMedia.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="🎬"
              title={allMedia.length === 0 ? 'No media found' : 'No matches'}
              description={
                allMedia.length === 0
                  ? 'Add a library and scan it to see your media here.'
                  : 'No media matches your filters. Try adjusting your search.'
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {filteredMedia.map((item: any) => (
            <MediaCard key={item.id} media={item} onClick={() => handlePlayMedia(item.id)} />
          ))}
        </div>
      )}

      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {filteredMedia.length} of {allMedia.length} items
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/media')({
  component: Media,
  validateSearch: (search: Record<string, unknown>) => {
    const id = search.id
    const parsedId = typeof id === 'string' ? parseInt(id, 10) : typeof id === 'number' ? id : undefined
    return {
      id: parsedId && !isNaN(parsedId) ? parsedId : undefined,
    }
  },
})
