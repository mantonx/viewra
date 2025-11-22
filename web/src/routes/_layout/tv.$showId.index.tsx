import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { Card, CardContent, Button } from '@/components/ui'
import { SeasonCard } from '@/components/tv'
import { PageHeader, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { tvApi } from '@/lib/api/tv'
import type { SeasonGroup } from '@/lib/types/tv'
import type { GithubComViewraViewraInternalApplicationTvTVEpisodeResponse } from '@/lib/api/generated/models'

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

  const allEpisodes = useMemo(() => (episodesData?.data && 'episodes' in episodesData.data) ? episodesData.data.episodes || [] : [], [episodesData])
  const isLoading = isLoadingShow || isLoadingEpisodes
  const error = showError || episodesError
  const showTitle = (showData?.data && 'title' in showData.data) ? showData.data.title || '' : ''

  // Group episodes by season
  const seasons = useMemo(() => {
    const seasonMap = new Map<number, SeasonGroup>()

    allEpisodes.forEach((episode: GithubComViewraViewraInternalApplicationTvTVEpisodeResponse) => {
      const seasonNum = episode.season ?? 0
      if (!seasonMap.has(seasonNum)) {
        seasonMap.set(seasonNum, {
          season: seasonNum,
          season_id: episode.season_id, // Use season_id from first episode
          episode_count: 0,
          episodes: [],
        })
      }
      const seasonGroup = seasonMap.get(seasonNum)
      if (seasonGroup && seasonGroup.episode_count !== undefined) {
        seasonGroup.episodes.push(episode)
        seasonGroup.episode_count++
      }
    })

    // Sort seasons (0 at the end, rest in ascending order)
    return Array.from(seasonMap.values()).sort((a, b) => {
      const aSeason = a.season ?? 0
      const bSeason = b.season ?? 0
      if (aSeason === 0) {return 1}
      if (bSeason === 0) {return -1}
      return aSeason - bSeason
    })
  }, [allEpisodes])

  const handleSeasonClick = (seasonNumber: number) => {
    navigate({ to: `/tv/${showId}/season/${seasonNumber}` })
  }

  const handleBackClick = () => {
    navigate({ to: '/tv', search: { q: undefined, sort: undefined, view: undefined } })
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
            key={season.season ?? 0}
            season={season}
            showTitle={showTitle}
            onClick={() => handleSeasonClick(season.season ?? 0)}
          />
        ))}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/tv/$showId/')({
  component: ShowDetail,
})
