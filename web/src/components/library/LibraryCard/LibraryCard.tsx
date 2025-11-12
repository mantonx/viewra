import {
  useLibrariesServiceDeleteApiLibrariesId,
  useLibrariesServicePostApiLibrariesIdScan,
} from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { Button } from '@/components/ui'
import type { LibraryCardProps } from './LibraryCard.types'

const LibraryCard = ({ library }: LibraryCardProps) => {
  const invalidateLibraries = useInvalidateLibraries()
  const deleteMutation = useLibrariesServiceDeleteApiLibrariesId()
  const scanMutation = useLibrariesServicePostApiLibrariesIdScan()

  const handleDelete = async () => {
    if (!library.id || !library.name) {
      return
    }
    if (!confirm(`Delete library "${library.name}"?`)) {
      return
    }

    try {
      await deleteMutation.mutateAsync({ id: library.id })
      invalidateLibraries()
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      alert(`Failed to delete library: ${message}`)
    }
  }

  const handleScan = async () => {
    if (!library.id) {
      return
    }
    try {
      await scanMutation.mutateAsync({ id: library.id })
      alert('Scan started successfully! The library will be updated shortly.')
      invalidateLibraries()
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown error'
      alert(`Failed to start scan: ${message}`)
    }
  }

  return (
    <div className="p-4 hover:bg-gray-50">
      <div className="flex justify-between items-start">
        <div className="flex-1">
          <h3 className="font-semibold text-lg">{library.name}</h3>
          <p className="text-sm text-gray-600">{library.path}</p>
          <div className="mt-2 flex gap-4 text-sm text-gray-500">
            <span>Type: {library.type}</span>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="success"
            size="sm"
            onClick={handleScan}
            isLoading={scanMutation.isPending}
          >
            Scan
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={handleDelete}
            isLoading={deleteMutation.isPending}
          >
            Delete
          </Button>
        </div>
      </div>
    </div>
  )
}

export { LibraryCard }
export type { LibraryCardProps } from './LibraryCard.types'
