import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '@/contexts'
import { Button, Input, Card, Alert } from '@/components/ui'

const SetupPage = () => {
  const navigate = useNavigate()
  const { isAuthenticated, isLoading: authLoading, needsSetup, login } = useAuth()

  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  // Redirect if already authenticated or setup not needed
  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      navigate({ to: '/' })
    }
  }, [isAuthenticated, authLoading, navigate])

  useEffect(() => {
    if (!authLoading && !needsSetup && !isAuthenticated) {
      navigate({ to: '/login' })
    }
  }, [needsSetup, authLoading, isAuthenticated, navigate])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validation
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    setIsLoading(true)

    try {
      // Create admin account
      const response = await fetch('/api/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username,
          password,
          display_name: displayName || username,
        }),
      })

      if (!response.ok) {
        const data = await response.json().catch(() => ({ error: 'Setup failed' }))
        throw new Error(data.error || 'Setup failed')
      }

      // Auto-login after setup
      await login(username, password)
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setIsLoading(false)
    }
  }

  if (authLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-neutral-100 dark:bg-neutral-950">
        <div className="animate-pulse text-neutral-500">Loading...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-neutral-100 dark:bg-neutral-950 p-4">
      <Card className="w-full max-w-md p-8">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-neutral-900 dark:text-neutral-50">
            Welcome to ViewRA
          </h1>
          <p className="text-neutral-600 dark:text-neutral-400 mt-2">
            Create your administrator account
          </p>
        </div>

        {error && (
          <Alert variant="error" className="mb-6">
            {error}
          </Alert>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <Input
            label="Username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
            autoFocus
            helperText="This will be used to sign in"
          />

          <Input
            label="Display Name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={username || 'Optional'}
            autoComplete="name"
          />

          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
            helperText="At least 8 characters"
          />

          <Input
            label="Confirm Password"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            autoComplete="new-password"
            error={confirmPassword && password !== confirmPassword ? 'Passwords do not match' : undefined}
          />

          <Button
            type="submit"
            className="w-full"
            isLoading={isLoading}
            disabled={!username || !password || !confirmPassword}
          >
            Create Account
          </Button>
        </form>
      </Card>
    </div>
  )
}

export const Route = createFileRoute('/setup')({
  component: SetupPage,
})
