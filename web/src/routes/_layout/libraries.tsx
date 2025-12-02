import { createFileRoute } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Button, Card, CardHeader, CardContent } from '@/components/ui'
import { LibraryCard, LibraryForm } from '@/components/library'
import { PageHeader, EmptyState, LoadingPage, ErrorPage } from '@/components/common'
import { extractLibraries } from '@/lib/utils/api'

const Libraries = () => {
  const [showCreateForm, setShowCreateForm] = useState(false)
  const { data: libraries, isLoading, error } = useQuery(getGetApiLibrariesQueryOptions())

  if (isLoading) {
    return <LoadingPage text="Loading libraries..." />
  }

  if (error) {
    return <ErrorPage error={error} context="libraries" />
  }

  const libraryList = extractLibraries(libraries)

  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="Libraries"
        description="Manage your media libraries. Add folders to scan for movies, TV shows, and music."
        actions={<Button onClick={() => setShowCreateForm(true)}>+ Add Library</Button>}
      />

      {showCreateForm && (
        <Card className="mb-6">
          <CardContent>
            <h2 className="text-lg font-semibold mb-4 text-neutral-900 dark:text-neutral-50">Create New Library</h2>
            <LibraryForm
              onCancel={() => setShowCreateForm(false)}
              onSuccess={() => setShowCreateForm(false)}
            />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold text-neutral-900 dark:text-neutral-50">Your Libraries</h2>
        </CardHeader>

        {libraryList.length === 0 ? (
          <CardContent>
            <EmptyState
              icon="📚"
              title="No libraries yet"
              description='Click the "+ Add Library" button to get started.'
            />
          </CardContent>
        ) : (
          <div className="divide-y">
            {libraryList.map((library) => (
              <LibraryCard key={library.id} library={library} />
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}

export const Route = createFileRoute('/_layout/libraries')({
  component: Libraries,
})
