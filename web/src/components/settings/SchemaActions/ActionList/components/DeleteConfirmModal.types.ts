/**
 * Types for DeleteConfirmModal component.
 */

export interface DeleteConfirmModalProps {
  /** Whether modal is open */
  isOpen: boolean
  /** Modal title */
  title: string
  /** Confirmation message */
  message: string
  /** Name of item being deleted (for display) */
  itemName: string
  /** Whether delete is in progress */
  isDeleting: boolean
  /** Called to confirm delete */
  onConfirm: () => void
  /** Called to cancel */
  onCancel: () => void
}
