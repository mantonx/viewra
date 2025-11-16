import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card, CardContent, Input, Select } from '@/components/ui'
import { ArtistCard } from '@/components/music'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { extractLibraries, getLibraryId, filterLibrariesByType } from '@/lib/utils/api'
import { musicApi } from '@/lib/api/music'

const Music = () => {
  const navigate = useNavigate()
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const libraries = extractLibraries(librariesData)
  
  // Filter to only music libraries
  const musicLibraries = filterLibrariesByType(libraries, 'music')

  // Get the library ID (defaults to first music library when 'all' is selected)
  const libraryId = getLibraryId(selectedLibrary, libraries, 'music')

  const {
    data: artistsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['music-artists', libraryId],
    queryFn: () => musicApi.listArtists(libraryId),
  })

  const allArtists = artistsData?.artists || []

  // Filter artists by search query
  const filteredArtists = allArtists.filter((artist) => {
    const matchesSearch =
      searchQuery === '' || artist.name.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesSearch
  })

  // Handle clicking on an artist card
  const handleArtistClick = (artistName: string) => {
    navigate({ to: `/music/${artistName}` })
  }

  if (isLoading) {
    return <LoadingPage text="Loading artists..." />
  }

  if (error) {
    return <ErrorPage error={error} context="music artists" />
  }

  return (
    <div className="p-8">
      <PageHeader title="Music" description="Browse your music collection by artist." />

      {/* Filters */}
      <Card className="mb-6">
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search artists..."
            />
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
          </div>
        </CardContent>
      </Card>

      {/* Artists Grid */}
      {filteredArtists.length === 0 ? (
        <Card>
          <CardContent>
            <EmptyState
              icon="🎤"
              title={allArtists.length === 0 ? 'No artists found' : 'No matches'}
              description={
                allArtists.length === 0
                  ? 'Add a library with music and scan it to see your artists here.'
                  : 'No artists match your search. Try adjusting your query.'
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {filteredArtists.map((artist) => (
            <ArtistCard
              key={artist.name}
              artist={artist}
              onClick={() => handleArtistClick(artist.name)}
            />
          ))}
        </div>
      )}

      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {filteredArtists.length} of {allArtists.length} artists
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/music/')({
  component: Music,
})
