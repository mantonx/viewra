import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { Card, CardContent, Button, Loading } from '@/components/ui'
import { EpisodeCard } from '@/components/tv'
import { VideoPlayer } from '@/components/media/VideoPlayer'
import { PageHeader, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { tvApi } from '@/lib/api/tv'
import { useMediaPlayback } from '@/lib/hooks/useMediaPlayback'
import type { TVEpisodeResponse } from '@/lib/types/tv'
import { logger } from '@/lib/utils/logger'

const SeasonDetail = () => {
  const navigate = useNavigate()
  const { showId, seasonNumber } = Route.useParams()
  const search = Route.useSearch() as { episodeId?: number }
  const urlEpisodeId = search.episodeId
  const showIdNumber = parseInt(showId, 10)

  const { playbackState, playMedia, stopPlayback } = useMediaPlayback()

  const {
    data: showData,
    isLoading: isLoadingShow,
    error: showError,
  } = useQuery({
    queryKey: ['tv-show', showIdNumber],
    queryFn: () => tvApi.getShow(showIdNumber),
  })

  const {
    data: episodesData,
    isLoading: isLoadingEpisodes,
    error: episodesError,
  } = useQuery({
    queryKey: ['tv-episodes', showIdNumber],
    queryFn: () => tvApi.listEpisodesByShowId(showIdNumber),
  })

  const allEpisodes = episodesData?.episodes || []
  const isLoading = isLoadingShow || isLoadingEpisodes
  const error = showError || episodesError
  const showTitle = showData?.title || ''

  // Filter episodes for this season and sort by episode number
  const seasonEpisodes = useMemo(() => {
    return allEpisodes
      .filter((ep) => ep.season === parseInt(seasonNumber, 10))
      .sort((a, b) => a.episode - b.episode)
  }, [allEpisodes, seasonNumber])

  // Find currently playing episode
  const playingEpisode = useMemo(
    () => seasonEpisodes.find((ep) => ep.id === playbackState.mediaId),
    [seasonEpisodes, playbackState.mediaId]
  )

  // Get next episode
  const nextEpisode = useMemo(() => {
    if (!playingEpisode) return null
    const currentIndex = seasonEpisodes.findIndex((ep) => ep.id === playingEpisode.id)
    if (currentIndex === -1 || currentIndex === seasonEpisodes.length - 1) return null
    return seasonEpisodes[currentIndex + 1]
  }, [playingEpisode, seasonEpisodes])

  // Auto-play episode if ID is in URL (only on initial load)
  useEffect(() => {
    if (urlEpisodeId && !playbackState.isPlaying && !playbackState.mediaId && seasonEpisodes.length > 0) {
      const episode = seasonEpisodes.find((ep) => ep.id === urlEpisodeId)
      if (episode) {
        handlePlayEpisode(episode)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [urlEpisodeId, seasonEpisodes.length])

  const handlePlayEpisode = async (episode: TVEpisodeResponse) => {
    logger.debug('Playing episode:', episode.show_title, 'S' + episode.season + 'E' + episode.episode)

    // Update URL with episode ID
    navigate({
      to: `/tv/${showId}/season/${seasonNumber}`,
      search: { episodeId: episode.id }
    })

    // Trigger playback
    await playMedia(episode.id, episode)
  }

  const handlePlayNextEpisode = async () => {
    if (nextEpisode) {
      await handlePlayEpisode(nextEpisode)
    }
  }

  const handleClosePlayer = () => {
    stopPlayback()
    // Clear URL parameter if present
    if (urlEpisodeId) {
      navigate({
        to: `/tv/${showId}/season/${seasonNumber}`,
        search: {}
      })
    }
  }

  const handleBackClick = () => {
    navigate({ to: `/tv/${showId}` })
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

  // If video player is showing, render it with next episode button
  if (playbackState.isPlaying && playbackState.streamUrl && playingEpisode) {
    return (
      <div className="relative">
        <VideoPlayer
          mediaId={playingEpisode.id}
          streamUrl={playbackState.streamUrl}
          initialPosition={playbackState.initialPosition}
          duration={playingEpisode.duration}
          onClose={handleClosePlayer}
        />
        {/* Next Episode Button Overlay */}
        {nextEpisode && (
          <div className="fixed bottom-24 right-8 z-50">
            <button
              onClick={handlePlayNextEpisode}
              className="bg-indigo-600 text-white px-6 py-3 rounded-lg shadow-lg hover:bg-indigo-700 transition-colors flex items-center gap-2 font-semibold"
            >
              <span>Next Episode</span>
              <span>→</span>
            </button>
          </div>
        )}
      </div>
    )
  }

  if (isLoading) {
    return <LoadingPage text="Loading season..." />
  }

  if (error) {
    return <ErrorPage error={error} context="season" />
  }

  if (seasonEpisodes.length === 0) {
    return (
      <div className="p-8">
        <PageHeader
          title={`${showTitle} - Season ${seasonNumber}`}
          description="No episodes found"
          actions={<Button onClick={handleBackClick}>← Back to Show</Button>}
        />
        <Card>
          <CardContent>
            <EmptyState
              icon="📺"
              title="No episodes found"
              description="This season doesn't have any episodes in the selected library."
            />
          </CardContent>
        </Card>
      </div>
    )
  }

  const seasonLabel = parseInt(seasonNumber, 10) === 0 ? 'Specials' : `Season ${seasonNumber}`

  return (
    <div className="p-8">
      <PageHeader
        title={`${showTitle} - ${seasonLabel}`}
        description={`${seasonEpisodes.length} ${seasonEpisodes.length === 1 ? 'Episode' : 'Episodes'}`}
        actions={<Button onClick={handleBackClick}>← Back to Show</Button>}
      />

      {/* Episodes Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
        {seasonEpisodes.map((episode) => (
          <EpisodeCard
            key={episode.id}
            episode={episode}
            onClick={() => handlePlayEpisode(episode)}
          />
        ))}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/tv/$showId/season/$seasonNumber')({
  component: SeasonDetail,
  validateSearch: (search: Record<string, unknown>) => {
    const episodeId = search.episodeId
    const parsedId = typeof episodeId === 'string' ? parseInt(episodeId, 10) : typeof episodeId === 'number' ? episodeId : undefined
    return {
      episodeId: parsedId && !isNaN(parsedId) ? parsedId : undefined,
    }
  },
})
