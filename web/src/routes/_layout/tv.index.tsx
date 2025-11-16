import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card, CardContent, Input, Select } from '@/components/ui'
import { TVShowCard } from '@/components/tv'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { extractLibraries, getLibraryId, filterLibrariesByType } from '@/lib/utils/api'
import { tvApi } from '@/lib/api/tv'

const TVShows = () => {
  const navigate = useNavigate()
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const libraries = extractLibraries(librariesData)

  // Filter to only TV libraries
  const tvLibraries = filterLibrariesByType(libraries, 'tv')

  // Get the library ID (defaults to first TV library when 'all' is selected)
  const libraryId = getLibraryId(selectedLibrary, libraries, 'tv')

  const {
    data: tvShowsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['tv-shows', libraryId],
    queryFn: () => tvApi.listShows(libraryId),
  })

  const allShows = tvShowsData?.shows || []

  // Filter shows by search query
  const filteredShows = allShows.filter((show) => {
    const matchesSearch =
      searchQuery === '' || show.title.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesSearch
  })

  // Handle clicking on a show card
  const handleShowClick = (showId: number) => {
    navigate({ to: `/tv/${showId}` })
  }

  if (isLoading) {
    return <LoadingPage text="Loading TV shows..." />
  }

  if (error) {
    return <ErrorPage error={error} context="TV shows" />
  }

  return (
    <div className="p-8">
      <PageHeader title="TV Shows" description="Browse your TV show collection." />

      {/* Filters */}
      <Card className="mb-6">
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search TV shows..."
            />
            <Select
              label="Library"
              value={selectedLibrary}
              onChange={(e) => setSelectedLibrary(e.target.value)}
              options={[
                { value: 'all', label: 'All TV Libraries' },
                ...tvLibraries.map((lib) => ({
                  value: String(lib.id),
                  label: lib.name || '',
                })),
              ]}
            />
          </div>
        </CardContent>
      </Card>

      {/* TV Shows Grid */}
      {filteredShows.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="📺"
              title={allShows.length === 0 ? 'No TV shows found' : 'No matches'}
              description={
                allShows.length === 0
                  ? 'Add a library with TV shows and scan it to see your shows here.'
                  : 'No TV shows match your search. Try adjusting your query.'
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {filteredShows.map((show) => (
            <TVShowCard
              key={show.id}
              show={show}
              onClick={() => handleShowClick(show.id)}
            />
          ))}
        </div>
      )}

      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {filteredShows.length} of {allShows.length} shows
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/tv/')({
  component: TVShows,
})
