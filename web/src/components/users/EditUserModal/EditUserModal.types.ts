import type { GithubComMantonxViewraInternalApplicationAuthUserResponse as User } from '@/lib/api/generated/models'

export type EditUserModalProps = {
  isOpen: boolean
  onClose: () => void
  user: User
  currentUserId?: string
  onSuccess: (updatedUser: User) => void
}
