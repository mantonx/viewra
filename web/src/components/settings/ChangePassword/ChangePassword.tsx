import { useState, type FormEvent } from 'react'
import { Card, CardHeader, CardContent, Button, Input, Alert } from '@/components/ui'
import { authFetch } from '@/lib/utils/authFetch'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import type { ChangePasswordProps } from './ChangePassword.types'

export const ChangePassword = ({ className }: ChangePasswordProps) => {
  const toast = useToast()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    if (newPassword !== confirmPassword) {
      setError('New passwords do not match')
      return
    }

    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters')
      return
    }

    setIsLoading(true)

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
      setError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Card className={className}>
      <CardHeader>
        <h2 className={cn('text-lg font-semibold', text.primary)}>Change Password</h2>
        <p className={cn('text-sm mt-1', text.secondary)}>
          Update your password to keep your account secure
        </p>
      </CardHeader>
      <CardContent>
        {error && (
          <Alert variant="error" className="mb-4">
            {error}
          </Alert>
        )}

        <form onSubmit={handleSubmit} className="space-y-4 max-w-md">
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
            error={
              confirmPassword && newPassword !== confirmPassword
                ? 'Passwords do not match'
                : undefined
            }
          />

          <Button
            type="submit"
            isLoading={isLoading}
            disabled={!currentPassword || !newPassword || !confirmPassword}
          >
            Change Password
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
