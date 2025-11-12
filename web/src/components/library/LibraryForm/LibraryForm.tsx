import { useState } from 'react'
import { useLibrariesServicePostApiLibraries } from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { Button, Input, Select, Alert } from '@/components/ui'
import type { LibraryFormProps } from './LibraryForm.types'

const LibraryForm = ({ onCancel, onSuccess }: LibraryFormProps) => {
  const invalidateLibraries = useInvalidateLibraries()
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [type, setType] = useState<'movie' | 'tv' | 'music'>('movie')
  const [error, setError] = useState<string | null>(null)

  const createMutation = useLibrariesServicePostApiLibraries()

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
      setType('movie')

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

      <Input
        label="Folder Path"
        value={path}
        onChange={(e) => setPath(e.target.value)}
        placeholder="/media/movies"
        helperText="Full path to the folder containing your media files"
        required
        disabled={createMutation.isPending}
      />

      <Select
        label="Library Type"
        value={type}
        onChange={(e) => setType(e.target.value as 'movie' | 'tv' | 'music')}
        options={[
          { value: 'movie', label: 'Movies' },
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
    </form>
  )
}

export { LibraryForm }
export type { LibraryFormProps } from './LibraryForm.types'
