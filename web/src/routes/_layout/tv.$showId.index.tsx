import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { Card, CardContent, Button } from '@/components/ui'
import { SeasonCard } from '@/components/tv'
import { PageHeader, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { tvApi } from '@/lib/api/tv'
import type { SeasonGroup } from '@/lib/types/tv'

const ShowDetail = () => {
  const navigate = useNavigate()
  const { showId } = Route.useParams()
  const showIdNumber = parseInt(showId, 10)

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

  // Group episodes by season
  const seasons = useMemo(() => {
    const seasonMap = new Map<number, SeasonGroup>()

    allEpisodes.forEach((episode) => {
      if (!seasonMap.has(episode.season)) {
        seasonMap.set(episode.season, {
          season: episode.season,
          season_id: episode.season_id, // Use season_id from first episode
          episode_count: 0,
          episodes: [],
        })
      }
      const seasonGroup = seasonMap.get(episode.season)
      if (seasonGroup) {
        seasonGroup.episodes.push(episode)
        seasonGroup.episode_count++
      }
    })

    // Sort seasons (0 at the end, rest in ascending order)
    return Array.from(seasonMap.values()).sort((a, b) => {
      if (a.season === 0) {return 1}
      if (b.season === 0) {return -1}
      return a.season - b.season
    })
  }, [allEpisodes])

  const handleSeasonClick = (seasonNumber: number) => {
    navigate({ to: `/tv/${showId}/season/${seasonNumber}` })
  }

  const handleBackClick = () => {
    navigate({ to: '/tv', search: {} })
  }

  if (isLoading) {
    return <LoadingPage text="Loading show details..." />
  }

  if (error) {
    return <ErrorPage error={error} context="show details" />
  }

  if (allEpisodes.length === 0) {
    return (
      <div className="p-8">
        <PageHeader
          title={showTitle}
          description="No episodes found"
          actions={<Button onClick={handleBackClick}>← Back to TV Shows</Button>}
        />
        <Card>
          <CardContent>
            <EmptyState
              icon="📺"
              title="No episodes found"
              description="This show doesn't have any episodes in the selected library."
            />
          </CardContent>
        </Card>
      </div>
    )
  }

  const totalEpisodes = allEpisodes.length

  return (
    <div className="p-8">
      <PageHeader
        title={showTitle}
        description={`${seasons.length} ${seasons.length === 1 ? 'Season' : 'Seasons'} • ${totalEpisodes} ${totalEpisodes === 1 ? 'Episode' : 'Episodes'}`}
        actions={<Button onClick={handleBackClick}>← Back to TV Shows</Button>}
      />

      {/* Seasons Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
        {seasons.map((season) => (
          <SeasonCard
            key={season.season}
            season={season}
            showTitle={showTitle}
            onClick={() => handleSeasonClick(season.season)}
          />
        ))}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/tv/$showId/')({
  component: ShowDetail,
})
