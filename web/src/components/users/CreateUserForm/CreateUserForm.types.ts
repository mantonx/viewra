import type { GithubComMantonxViewraInternalApplicationAuthUserResponse as User } from '@/lib/api/generated/models'

export type CreateUserFormProps = {
  onSuccess: (user: User) => void
  onCancel: () => void
}
