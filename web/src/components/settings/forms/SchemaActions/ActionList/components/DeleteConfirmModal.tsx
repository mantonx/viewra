import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Trash2 } from 'lucide-react'
import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import type { DeleteConfirmModalProps } from './DeleteConfirmModal.types'

export const DeleteConfirmModal = ({
  isOpen,
  title,
  message,
  itemName,
  isDeleting,
  onConfirm,
  onCancel,
}: DeleteConfirmModalProps) => {
  const displayMessage = message.replace('{name}', itemName)

  return (
    <Modal isOpen={isOpen} onClose={onCancel} title={title}>
      <ModalContent>
        <div className="flex items-start gap-4">
          <div
            className={cn(
              'p-2 rounded-full',
              'bg-red-100 dark:bg-red-900/50',
              'text-red-600 dark:text-red-400'
            )}
          >
            <Trash2 className="w-5 h-5" />
          </div>
          <div>
            <p className={cn('text-sm', text.secondary)}>
              {displayMessage}
            </p>
          </div>
        </div>
      </ModalContent>
      <ModalFooter>
        <Button variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="danger" onClick={onConfirm} isLoading={isDeleting}>
          <Trash2 className="w-4 h-4 mr-1" />
          Delete
        </Button>
      </ModalFooter>
    </Modal>
  )
}

DeleteConfirmModal.displayName = 'DeleteConfirmModal'
