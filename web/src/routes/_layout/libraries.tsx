import { createFileRoute } from '@tanstack/react-router'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Button } from '@/components/ui'
import { LibraryCard, LibraryForm } from '@/components/library'
import { StageStatus } from '@/components/enrichment/StageStatus'
import { SettingsPage, LoadingPage, ErrorPage, EmptyState } from '@/components/common'
import { extractLibraries } from '@/lib/utils/api'

const Libraries = () => {
  const [showCreateForm, setShowCreateForm] = useState(false)
  const {
    data: libraries,
    isLoading,
    error,
  } = useQuery(getGetApiLibrariesQueryOptions())

  if (isLoading) {
    return <LoadingPage text="Loading libraries..." />
  }

  if (error) {
    return <ErrorPage error={error} context="libraries" />
  }

  const libraryList = extractLibraries(libraries)

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Libraries"
        description="Manage your media libraries. Add folders to scan for movies, TV shows, and music."
        actions={<Button onClick={() => setShowCreateForm(true)}>+ Add Library</Button>}
      />

      {/* Circuit breaker status - shows only when there are problems */}
      <StageStatus showOnlyProblems />

      {/* Create library form */}
      {showCreateForm && (
        <SettingsPage.Card title="Create New Library" className="mb-6">
          <LibraryForm
            onCancel={() => setShowCreateForm(false)}
            onSuccess={() => setShowCreateForm(false)}
          />
        </SettingsPage.Card>
      )}

      {/* Libraries list */}
      <SettingsPage.Card title="Your Libraries">
        {libraryList.length === 0 ? (
          <EmptyState
            icon="library"
            title="No libraries yet"
            description='Click the "+ Add Library" button to get started.'
          />
        ) : (
          <SettingsPage.List>
            {libraryList.map((library) => (
              <LibraryCard key={library.id} library={library} />
            ))}
          </SettingsPage.List>
        )}
      </SettingsPage.Card>
    </SettingsPage>
  )
}

export const Route = createFileRoute('/_layout/libraries')({
  component: Libraries,
})
