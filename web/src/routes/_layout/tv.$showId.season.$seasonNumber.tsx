import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { Card, CardContent, Button } from '@/components/ui'
import { EpisodeCard } from '@/components/tv'
import { VideoPlayerContainer } from '@/components/media'
import { PageHeader, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { tvApi } from '@/lib/api/tv'
import { useMediaPlayback, BatchProgressProvider } from '@/lib/hooks'
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

  const allEpisodes = useMemo(() => episodesData?.data?.episodes || [], [episodesData])
  const isLoading = isLoadingShow || isLoadingEpisodes
  const error = showError || episodesError
  const showTitle = showData?.data?.title || ''

  // Filter episodes for this season and sort by episode number
  const seasonEpisodes = useMemo(() => {
    return allEpisodes
      .filter((ep) => ep.season === parseInt(seasonNumber, 10))
      .sort((a, b) => a.episode - b.episode)
  }, [allEpisodes, seasonNumber])

  // Find currently playing episode and enrich with show title and show_id
  const playingEpisode = useMemo(() => {
    const episode = seasonEpisodes.find((ep) => ep.id === playbackState.mediaId)
    if (!episode) {return undefined}
    // Enrich episode with show metadata for video player
    return {
      ...episode,
      show_title: showTitle,
      show_id: showIdNumber,
    }
  }, [seasonEpisodes, playbackState.mediaId, showTitle, showIdNumber])

  // Get next episode
  const nextEpisode = useMemo(() => {
    if (!playingEpisode) {return null}
    const currentIndex = seasonEpisodes.findIndex((ep) => ep.id === playingEpisode.id)
    if (currentIndex === -1 || currentIndex === seasonEpisodes.length - 1) {return null}
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
    logger.debug('Playing episode:', episode.show_title, `S${  episode.season  }E${  episode.episode}`)

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

  // Render video player with next episode button overlay
  const nextEpisodeButton = nextEpisode ? (
    <div className="fixed bottom-24 right-8 z-50">
      <button
        onClick={handlePlayNextEpisode}
        className="bg-indigo-600 text-white px-6 py-3 rounded-lg shadow-lg hover:bg-indigo-700 transition-colors flex items-center gap-2 font-semibold"
      >
        <span>Next Episode</span>
        <span>→</span>
      </button>
    </div>
  ) : undefined

  const videoPlayer = (
    <VideoPlayerContainer
      playbackState={playbackState}
      media={playingEpisode}
      onClose={handleClosePlayer}
      overlay={nextEpisodeButton}
    />
  )

  if (playbackState.isPlaying && playingEpisode) {
    return videoPlayer
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
  const episodeIds = seasonEpisodes.map((ep) => ep.id)

  return (
    <div className="p-8">
      <PageHeader
        title={`${showTitle} - ${seasonLabel}`}
        description={`${seasonEpisodes.length} ${seasonEpisodes.length === 1 ? 'Episode' : 'Episodes'}`}
        actions={<Button onClick={handleBackClick}>← Back to Show</Button>}
      />

      {/* Episodes Grid */}
      <BatchProgressProvider mediaIds={episodeIds}>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
          {seasonEpisodes.map((episode) => (
            <EpisodeCard
              key={episode.id}
              episode={episode}
              onClick={() => handlePlayEpisode(episode)}
            />
          ))}
        </div>
      </BatchProgressProvider>
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
