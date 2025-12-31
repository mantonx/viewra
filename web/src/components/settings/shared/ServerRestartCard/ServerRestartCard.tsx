import { useState } from 'react'
import { Card, CardHeader, CardContent, Button } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { useServerStatus } from '@/contexts'
import { usePostApiAdminSystemRestartNow } from '@/lib/api/generated/system/system'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Power, RotateCcw } from 'lucide-react'

/**
 * Card for managing server restarts with pending changes display.
 * Shows when settings require a restart to take effect.
 */
export const ServerRestartCard = () => {
  const toast = useToast()
  const { setRestarting, restartPending, pendingSettings } = useServerStatus()
  const [showConfirm, setShowConfirm] = useState(false)

  const restartMutation = usePostApiAdminSystemRestartNow()

  const isPending = restartPending

  const handleRestart = async () => {
    try {
      setRestarting() // Trigger full-screen overlay
      await restartMutation.mutateAsync()
      setShowConfirm(false)
    } catch {
      toast.error('Failed to restart server')
    }
  }

  return (
    <Card variant="glass">
      <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className={cn(
                'p-2 rounded-lg',
                'bg-neutral-100 dark:bg-neutral-800',
                'text-neutral-600 dark:text-neutral-400'
              )}
            >
              <Power className="w-5 h-5" />
            </div>
            <div>
              <h2 className={cn('text-lg font-semibold', text.primary)}>Server Control</h2>
              <p className={cn('text-sm mt-0.5', text.secondary)}>
                Manage server state and apply pending changes
              </p>
            </div>
          </div>

        </div>
      </CardHeader>

      <CardContent className="py-4">
        {isPending && pendingSettings && pendingSettings.length > 0 && (
          <div className="mb-4 p-3 rounded-lg bg-neutral-50 dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-700">
            <p className={cn('text-xs font-medium uppercase tracking-wider mb-2', text.tertiary)}>
              Pending changes
            </p>
            <div className="flex flex-wrap gap-1.5">
              {pendingSettings.map((setting: string) => (
                <span
                  key={setting}
                  className={cn(
                    'px-2 py-0.5 rounded text-xs font-mono',
                    'bg-neutral-200 dark:bg-neutral-800',
                    text.secondary
                  )}
                >
                  {setting}
                </span>
              ))}
            </div>
          </div>
        )}

        <div className="flex items-center justify-between">
          <div className="flex-1 mr-4">
            {showConfirm ? (
              <p className={cn('text-sm', text.secondary)}>
                Active streams will be interrupted. The server will be back online in a few seconds.
              </p>
            ) : (
              <p className={cn('text-sm', text.secondary)}>
                {isPending
                  ? 'Restart to apply the pending configuration changes.'
                  : 'Restart the server to apply configuration changes.'}
              </p>
            )}
          </div>
          {showConfirm ? (
            <div className="flex gap-2 shrink-0">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowConfirm(false)}
                disabled={restartMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleRestart}
                isLoading={restartMutation.isPending}
              >
                Confirm Restart
              </Button>
            </div>
          ) : (
            <Button
              variant={isPending ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setShowConfirm(true)}
              className="shrink-0"
            >
              <RotateCcw className="w-4 h-4 mr-1.5" />
              Restart Server
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
