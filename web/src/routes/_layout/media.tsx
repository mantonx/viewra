import { createFileRoute } from '@tanstack/react-router'
import { getGetApiMediaQueryOptions, getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card, CardContent, Input, Select, Alert, Loading } from '@/components/ui'
import { MediaCard } from '@/components/media'
import { PageHeader, EmptyState } from '@/components/common'

const Media = () => {
  const [selectedLibrary, setSelectedLibrary] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const {
    data: mediaData,
    isLoading: isLoadingMedia,
    error: mediaError,
  } = useQuery(getGetApiMediaQueryOptions())
  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())

  const libraries = (librariesData && 'data' in librariesData && librariesData.data && 'libraries' in librariesData.data ? librariesData.data.libraries : []) || []
  const allMedia = (mediaData && 'data' in mediaData && mediaData.data && 'media' in mediaData.data ? mediaData.data.media : []) || []

  // Filter media by selected library and search query
  const filteredMedia = allMedia.filter((item: any) => {
    const matchesLibrary = selectedLibrary === 'all' || item.library_id === selectedLibrary
    const matchesSearch =
      searchQuery === '' || item.title.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesLibrary && matchesSearch
  })

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
            <MediaCard key={item.id} media={item} />
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
})
