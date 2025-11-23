import { Button } from '@/components/ui/Button'
import { Modal, ModalContent, ModalFooter } from '@/components/ui/Modal'
import { useEffect, useRef } from 'react'
import type { ConfirmDialogProps } from './ConfirmDialog.types'

export const ConfirmDialog = ({
  isOpen,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'primary',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) => {
  const confirmButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isOpen) {
        return
      }

      if (e.key === 'Enter') {
        e.preventDefault()
        onConfirm()
      } else if (e.key === 'Escape') {
        e.preventDefault()
        onCancel()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onConfirm, onCancel])

  useEffect(() => {
    if (isOpen && confirmButtonRef.current) {
      confirmButtonRef.current.focus()
    }
  }, [isOpen])

  return (
    <Modal isOpen={isOpen} onClose={onCancel} title={title} size="sm">
      <ModalContent>
        <p className="text-neutral-700 dark:text-neutral-300">{message}</p>
      </ModalContent>
      <ModalFooter>
        <Button variant="secondary" onClick={onCancel}>
          {cancelText}
        </Button>
        <Button ref={confirmButtonRef} variant={variant} onClick={onConfirm}>
          {confirmText}
        </Button>
      </ModalFooter>
    </Modal>
  )
}
