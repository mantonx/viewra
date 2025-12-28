export type ResetPasswordModalProps = {
  isOpen: boolean
  onClose: () => void
  userId: string
  username: string
  onSuccess: () => void
}
