import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '@/contexts'
import { authFetch } from '@/lib/utils/authFetch'
import { Card, CardHeader, CardContent, Button, Input, Alert, Loading } from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Trash2, Monitor, Smartphone, Globe } from 'lucide-react'
import type { GithubComMantonxViewraInternalApplicationAuthSessionResponse as Session } from '@/lib/api/generated/models'

const AccountSettings = () => {
  const { user } = useAuth()
  const toast = useToast()

  // Password change state
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordLoading, setPasswordLoading] = useState(false)

  // Sessions state
  const [sessions, setSessions] = useState<Session[]>([])
  const [sessionsLoading, setSessionsLoading] = useState(true)
  const [sessionsError, setSessionsError] = useState<string | null>(null)

  // Fetch sessions on mount
  useEffect(() => {
    const fetchSessions = async () => {
      try {
        const response = await authFetch('/api/auth/sessions')
        if (!response.ok) {
          throw new Error('Failed to fetch sessions')
        }
        const data = await response.json()
        setSessions(data.sessions || [])
      } catch (err) {
        setSessionsError(err instanceof Error ? err.message : 'Failed to load sessions')
      } finally {
        setSessionsLoading(false)
      }
    }

    fetchSessions()
  }, [])

  const handlePasswordChange = async (e: FormEvent) => {
    e.preventDefault()
    setPasswordError(null)

    if (newPassword !== confirmPassword) {
      setPasswordError('New passwords do not match')
      return
    }

    if (newPassword.length < 8) {
      setPasswordError('New password must be at least 8 characters')
      return
    }

    setPasswordLoading(true)

    try {
      const response = await authFetch('/api/auth/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })

      if (!response.ok) {
        const data = await response.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to change password')
      }

      toast.success('Password changed successfully')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setPasswordLoading(false)
    }
  }

  const handleRevokeSession = async (sessionId: string) => {
    try {
      const response = await authFetch(`/api/auth/sessions/${sessionId}`, {
        method: 'DELETE',
      })

      if (!response.ok) {
        throw new Error('Failed to revoke session')
      }

      setSessions(prev => prev.filter(s => s.id !== sessionId))
      toast.success('Session revoked')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revoke session')
    }
  }

  const getDeviceIcon = (userAgent: string | undefined) => {
    if (!userAgent) {
      return <Globe className="w-5 h-5" />
    }
    const ua = userAgent.toLowerCase()
    if (ua.includes('mobile') || ua.includes('android') || ua.includes('iphone')) {
      return <Smartphone className="w-5 h-5" />
    }
    return <Monitor className="w-5 h-5" />
  }

  const formatUserAgent = (userAgent: string | undefined) => {
    if (!userAgent) {
      return 'Unknown device'
    }
    // Extract browser name from user agent
    if (userAgent.includes('Firefox')) {
      return 'Firefox'
    }
    if (userAgent.includes('Chrome')) {
      return 'Chrome'
    }
    if (userAgent.includes('Safari')) {
      return 'Safari'
    }
    if (userAgent.includes('Edge')) {
      return 'Edge'
    }
    return userAgent.slice(0, 50) + (userAgent.length > 50 ? '...' : '')
  }

  const formatDate = (dateStr: string | undefined) => {
    if (!dateStr) {
      return 'Unknown'
    }
    return new Date(dateStr).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="Account Settings"
        description="Manage your account security and active sessions"
      />

      {/* Account Info */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Account Information
          </h2>
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
              <span className={cn(
                'px-2 py-0.5 text-xs font-medium rounded',
                user?.is_admin
                  ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400'
                  : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
              )}>
                {user?.is_admin ? 'Administrator' : 'User'}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Change Password */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Change Password
          </h2>
          <p className={cn('text-sm mt-1', text.secondary)}>
            Update your password to keep your account secure
          </p>
        </CardHeader>
        <CardContent>
          {passwordError && (
            <Alert variant="error" className="mb-4">
              {passwordError}
            </Alert>
          )}

          <form onSubmit={handlePasswordChange} className="space-y-4 max-w-md">
            <Input
              label="Current Password"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
              autoComplete="current-password"
            />

            <Input
              label="New Password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              autoComplete="new-password"
              helperText="At least 8 characters"
            />

            <Input
              label="Confirm New Password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
              error={confirmPassword && newPassword !== confirmPassword ? 'Passwords do not match' : undefined}
            />

            <Button
              type="submit"
              isLoading={passwordLoading}
              disabled={!currentPassword || !newPassword || !confirmPassword}
            >
              Change Password
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Active Sessions */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Active Sessions
          </h2>
          <p className={cn('text-sm mt-1', text.secondary)}>
            Manage devices where you're currently logged in
          </p>
        </CardHeader>
        <CardContent>
          {sessionsLoading ? (
            <Loading text="Loading sessions..." />
          ) : sessionsError ? (
            <Alert variant="error">{sessionsError}</Alert>
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
                    session.is_current && 'ring-2 ring-blue-500 ring-offset-2 dark:ring-offset-neutral-900'
                  )}
                >
                  <div className="flex items-center gap-3">
                    <div className={cn('p-2 rounded-full bg-neutral-200 dark:bg-neutral-700', text.secondary)}>
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
                        {session.ip_address || 'Unknown IP'} • Last active {formatDate(session.last_used_at)}
                      </div>
                    </div>
                  </div>

                  {!session.is_current && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => session.id && handleRevokeSession(session.id)}
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
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/account')({
  component: AccountSettings,
})
