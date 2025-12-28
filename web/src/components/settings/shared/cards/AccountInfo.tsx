import { Card, CardHeader, CardContent } from '@/components/ui'
import { useAuth } from '@/contexts'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { AccountInfoProps } from './AccountInfo.types'

export const AccountInfo = ({ className }: AccountInfoProps) => {
  const { user } = useAuth()

  return (
    <Card variant="glass" className={className}>
      <CardHeader>
        <h2 className={cn('text-lg font-semibold', text.primary)}>Account Information</h2>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex justify-between">
            <span className={text.secondary}>Username</span>
            <span className={text.primary}>{user?.username}</span>
          </div>
          <div className="flex justify-between">
            <span className={text.secondary}>Display Name</span>
            <span className={text.primary}>{user?.display_name || user?.username}</span>
          </div>
          <div className="flex justify-between">
            <span className={text.secondary}>Role</span>
            <span
              className={cn(
                'px-2 py-0.5 text-xs font-medium rounded',
                user?.is_admin
                  ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400'
                  : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
              )}
            >
              {user?.is_admin ? 'Administrator' : 'User'}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
