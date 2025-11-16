import { useDeleteApiLibrariesId, usePostApiLibrariesIdScan } from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useToast } from '@/lib/hooks/useToast'
import { useConfirm } from '@/lib/hooks/useConfirm'
import { getErrorMessage } from '@/lib/utils/error'
import { Button } from '@/components/ui'
import type { LibraryCardProps } from './LibraryCard.types'

const LibraryCard = ({ library }: LibraryCardProps) => {
  const invalidateLibraries = useInvalidateLibraries()
  const deleteMutation = useDeleteApiLibrariesId()
  const scanMutation = usePostApiLibrariesIdScan()
  const toast = useToast()
  const { confirm } = useConfirm()

  const handleDelete = async () => {
    if (!library.id || !library.name) {
      return
    }

    const confirmed = await confirm({
      title: 'Delete Library',
      message: `Are you sure you want to delete "${library.name}"? This action cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })

    if (!confirmed) {
      return
    }

    try {
      await deleteMutation.mutateAsync({ id: library.id })
      invalidateLibraries()
      toast.success('Library deleted successfully')
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to delete library'))
    }
  }

  const handleScan = async () => {
    if (!library.id) {
      return
    }
    try {
      await scanMutation.mutateAsync({ id: library.id })
      toast.success('Scan started successfully! The library will be updated shortly.')
      invalidateLibraries()
    } catch (error) {
      toast.error(getErrorMessage(error, 'Failed to start scan'))
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
