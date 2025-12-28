import { Card, CardHeader, CardContent, Button, Alert, Loading } from '@/components/ui'
import { useActiveSessions } from '@/lib/hooks'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Trash2, Monitor, Smartphone, Globe } from 'lucide-react'
import type { ActiveSessionsProps } from './ActiveSessions.types'

const formatUserAgent = (userAgent: string | undefined): string => {
  if (!userAgent) {return 'Unknown device'}
  if (userAgent.includes('Firefox')) {return 'Firefox'}
  if (userAgent.includes('Chrome')) {return 'Chrome'}
  if (userAgent.includes('Safari')) {return 'Safari'}
  if (userAgent.includes('Edge')) {return 'Edge'}
  return userAgent.slice(0, 50) + (userAgent.length > 50 ? '...' : '')
}

const formatDate = (dateStr: string | undefined): string => {
  if (!dateStr) {return 'Unknown'}
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const getDeviceIcon = (userAgent: string | undefined) => {
  if (!userAgent) {return <Globe className="w-5 h-5" />}
  const ua = userAgent.toLowerCase()
  if (ua.includes('mobile') || ua.includes('android') || ua.includes('iphone')) {
    return <Smartphone className="w-5 h-5" />
  }
  return <Monitor className="w-5 h-5" />
}

export const ActiveSessions = ({ className }: ActiveSessionsProps) => {
  const { sessions, isLoading, error, revokeSession } = useActiveSessions()

  return (
    <Card variant="glass" className={className}>
      <CardHeader>
        <h2 className={cn('text-lg font-semibold', text.primary)}>Active Sessions</h2>
        <p className={cn('text-sm mt-1', text.secondary)}>
          Manage devices where you're currently logged in
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Loading text="Loading sessions..." />
        ) : error ? (
          <Alert variant="error">{error}</Alert>
        ) : sessions.length === 0 ? (
          <div className={cn('text-sm', text.secondary)}>No active sessions found</div>
        ) : (
          <div className="space-y-3">
            {sessions.map((session) => (
              <div
                key={session.id}
                className={cn(
                  'flex items-center justify-between p-3 rounded-lg',
                  'bg-neutral-50 dark:bg-neutral-800/50',
                  session.is_current &&
                    'ring-2 ring-blue-500 ring-offset-2 dark:ring-offset-neutral-900'
                )}
              >
                <div className="flex items-center gap-3">
                  <div
                    className={cn(
                      'p-2 rounded-full bg-neutral-200 dark:bg-neutral-700',
                      text.secondary
                    )}
                  >
                    {getDeviceIcon(session.user_agent)}
                  </div>
                  <div>
                    <div className={cn('font-medium', text.primary)}>
                      {formatUserAgent(session.user_agent)}
                      {session.is_current && (
                        <span className="ml-2 text-xs text-blue-600 dark:text-blue-400">
                          (This device)
                        </span>
                      )}
                    </div>
                    <div className={cn('text-sm', text.secondary)}>
                      {session.ip_address || 'Unknown IP'} · Last active{' '}
                      {formatDate(session.last_used_at)}
                    </div>
                  </div>
                </div>

                {!session.is_current && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => session.id && revokeSession(session.id)}
                    className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
