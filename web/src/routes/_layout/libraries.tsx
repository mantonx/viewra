import { createFileRoute } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Button, Card, CardHeader, CardContent, Alert, Loading } from '@/components/ui'
import { LibraryCard, LibraryForm } from '@/components/library'
import { PageHeader, EmptyState } from '@/components/common'

const Libraries = () => {
  const [showCreateForm, setShowCreateForm] = useState(false)
  const { data: libraries, isLoading, error } = useQuery(getGetApiLibrariesQueryOptions())

  if (isLoading) {
    return (
      <div className="p-8">
        <Loading size="lg" text="Loading libraries..." />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8">
        <Alert variant="error">Error loading libraries: {error.message}</Alert>
      </div>
    )
  }

  const libraryList = (libraries && 'data' in libraries && libraries.data && 'libraries' in libraries.data ? libraries.data.libraries : []) || []

  return (
    <div className="p-8">
      <PageHeader
        title="Libraries"
        description="Manage your media libraries. Add folders to scan for movies, TV shows, and music."
        actions={<Button onClick={() => setShowCreateForm(true)}>+ Add Library</Button>}
      />

      {showCreateForm && (
        <Card className="mb-6">
          <CardContent>
            <h2 className="text-lg font-semibold mb-4">Create New Library</h2>
            <LibraryForm
              onCancel={() => setShowCreateForm(false)}
              onSuccess={() => setShowCreateForm(false)}
            />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold">Your Libraries</h2>
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
            {libraryList.map((library: any) => (
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
