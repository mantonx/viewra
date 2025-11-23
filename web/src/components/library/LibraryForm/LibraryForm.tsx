import { useState } from 'react'
import { usePostApiLibraries } from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { Button, Input, Select, Alert } from '@/components/ui'
import { FilesystemBrowser } from '@/components/library/FilesystemBrowser'
import type { LibraryFormProps } from './LibraryForm.types'

const LibraryForm = ({ onCancel, onSuccess }: LibraryFormProps) => {
  const invalidateLibraries = useInvalidateLibraries()
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [type, setType] = useState<'movies' | 'tv' | 'music'>('movies')
  const [error, setError] = useState<string | null>(null)
  const [isBrowserOpen, setIsBrowserOpen] = useState(false)

  const createMutation = usePostApiLibraries()

  const handleBrowseClick = () => {
    setIsBrowserOpen(true)
  }

  const handlePathSelect = (selectedPath: string) => {
    setPath(selectedPath)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    try {
      await createMutation.mutateAsync({
        data: {
          name,
          path,
          type,
        },
      })

      invalidateLibraries()

      // Reset form
      setName('')
      setPath('')
      setType('movies')

      // Close form
      onSuccess()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create library'
      setError(message)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error && <Alert variant="error">{error}</Alert>}

      <Input
        label="Library Name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="My Movies"
        required
        disabled={createMutation.isPending}
      />

      <div>
        <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1">
          Folder Path
        </label>
        <div className="flex gap-2">
          <Input
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder="/media/movies"
            required
            disabled={createMutation.isPending}
            className="flex-1"
          />
          <Button
            type="button"
            variant="secondary"
            onClick={handleBrowseClick}
            disabled={createMutation.isPending}
          >
            Browse...
          </Button>
        </div>
        <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-500">
          Full path to the folder containing your media files
        </p>
      </div>

      <Select
        label="Library Type"
        value={type}
        onChange={(e) => setType(e.target.value as 'movies' | 'tv' | 'music')}
        options={[
          { value: 'movies', label: 'Movies' },
          { value: 'tv', label: 'TV Shows' },
          { value: 'music', label: 'Music' },
        ]}
        disabled={createMutation.isPending}
      />

      <div className="flex gap-2 justify-end">
        <Button
          type="button"
          variant="secondary"
          onClick={onCancel}
          disabled={createMutation.isPending}
        >
          Cancel
        </Button>
        <Button type="submit" isLoading={createMutation.isPending}>
          Create Library
        </Button>
      </div>

      <FilesystemBrowser
        isOpen={isBrowserOpen}
        onClose={() => setIsBrowserOpen(false)}
        onSelect={handlePathSelect}
        initialPath={path || undefined}
      />
    </form>
  )
}

export { LibraryForm }
export type { LibraryFormProps } from './LibraryForm.types'
